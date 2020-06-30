package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"time"

	"github.com/hashgraph/hedera-sdk-go"
	"github.com/spf13/cobra"
)

var (
	memo               string
	adminKeys          *[]string
	adminExclude       bool
	adminKeyThreshold  uint32
	submitKeys         *[]string
	submitExclude      bool
	submitKeyThreshold uint32

	unixEpoch = time.Unix(0, 0)
)

func buildTopicCommand() *cobra.Command {
	createTopicCmd := &cobra.Command{
		Use:  "create [count]",
		Short: "Create HCS Topics",
		Args: cobra.MaximumNArgs(1),
		RunE: createTopic,
	}

	createTopicCmd.Flags().StringVar(&memo, "memo", "topic created by hcscli", "memor for the topic")
	adminKeys = createTopicCmd.Flags().StringArray("admin-key", nil, "public key of admin, default to the operator public key")
	createTopicCmd.Flags().BoolVar(&adminExclude, "admin-exclude", false, "exclude the operator from admin")
	createTopicCmd.Flags().Uint32Var(&adminKeyThreshold, "admin-key-threshold", 1, "threshold of admin key")
	submitKeys = createTopicCmd.Flags().StringArray("submit-key", nil, "public key of account allowed to submit messages")
	createTopicCmd.Flags().BoolVar(&submitExclude, "submit-exclude", false, "exclude the operator from submitting messages")
	createTopicCmd.Flags().Uint32Var(&submitKeyThreshold, "submit-key-threshold", 1, "threshold of submit key")

	deleteTopicCmd := &cobra.Command{
		Use:  "delete [topic ID]",
		Short: "Delete an HCS topic",
		Args: cobra.ExactArgs(1),
		RunE: deleteTopic,
	}

	queryTopicCmd := &cobra.Command{
		Use:  "query [topic ID]",
		Short: "Query info about an HCS topic",
		Args: cobra.ExactArgs(1),
		RunE: queryTopic,
	}

	subscribeTopicCmd := &cobra.Command{
		Use:  "subscribe [topic ID] [count]",
		Short: "Subscribe to an HCS topic to receive responses",
		Args: cobra.RangeArgs(1, 2),
		RunE: subscribeTopic,
	}

	benchmarkTopicCmd := &cobra.Command{
		Use:  "benchmark [topic ID] [count]",
		Short: "Benchmark an HCS topic",
		Args: cobra.RangeArgs(1, 2),
		RunE: benchmarkTopic,
	}

	topicCmd := &cobra.Command{
		Use:   "topic",
		Short: "Topic related commands",
	}

	topicCmd.AddCommand(createTopicCmd)
	topicCmd.AddCommand(deleteTopicCmd)
	topicCmd.AddCommand(queryTopicCmd)
	topicCmd.AddCommand(subscribeTopicCmd)
	topicCmd.AddCommand(benchmarkTopicCmd)

	return topicCmd
}

func createTopic(cmd *cobra.Command, args []string) (err error) {
	count := 1
	if len(args) != 0 {
		if count, err = strconv.Atoi(args[0]); err != nil {
			return fmt.Errorf("invalid count: %s", err)
		}
		if count < 1 || count > 10 {
			return fmt.Errorf("count must be in the range [1, 10], got %d", count)
		}
	}

	operatorPublicKey := config.operatorKey.PublicKey()
	adminKey, err := makePublicKey(adminKeyThreshold, *adminKeys, operatorPublicKey, adminExclude)
	if err != nil {
		return fmt.Errorf("failed to make admin key: %s", err)
	}

	submitKey, err := makePublicKey(submitKeyThreshold, *submitKeys, operatorPublicKey, submitExclude)
	if err != nil {
		return fmt.Errorf("failed to make submit key: %s", err)
	}

	client := createClient()

	for i := 0; i < count; i++ {
		tx := hedera.NewConsensusTopicCreateTransaction().SetTopicMemo(memo)

		if adminKey != nil {
			tx.SetAdminKey(adminKey)
		}

		if submitKey != nil {
			tx.SetSubmitKey(submitKey)
		}

		txId, err := tx.Execute(client)
		if err != nil {
			return fmt.Errorf("failed to create a new topic: %s", err)
		}

		receipt, err := txId.GetReceipt(client)
		if err != nil {
			return fmt.Errorf("failed to get receipt of topic create transaction: %s", err)
		}

		fmt.Printf("new topic id: %s\n", receipt.GetConsensusTopicID())
	}

	return nil
}

func deleteTopic(cmd *cobra.Command, args []string) error {
	topicID, err := hedera.TopicIDFromString(args[0])
	if err != nil {
		return fmt.Errorf("invalid topic ID: %s", err)
	}

	client := createClient()

	txID, err := hedera.NewConsensusTopicDeleteTransaction().SetTopicID(topicID).Execute(client)
	if err != nil {
		return fmt.Errorf("failed to run the topic delete transaction: %s", err)
	}

	if _, err := txID.GetReceipt(client); err != nil {
		return fmt.Errorf("failed to get receipt for the topic delete transaction: %s", err)
	}

	fmt.Println(topicID, "is deleted")
	return nil
}

func queryTopic(cmd *cobra.Command, args []string) error {
	topicID, err := hedera.TopicIDFromString(args[0])
	if err != nil {
		return fmt.Errorf("invalid topic ID: %s", err)
	}

	client := createClient()

	info, err := hedera.NewConsensusTopicInfoQuery().SetTopicID(topicID).Execute(client)
	if err != nil {
		return fmt.Errorf("failed to query topic: %s", err)
	}

	fmt.Printf("Topic - %s, info:\n", topicID)
	fmt.Printf("  Memo - %s\n", info.Memo)
	fmt.Printf("  RunningHash (hex) - %s\n", hex.EncodeToString(info.RunningHash))
	fmt.Printf("  RunningHash (base64) - %s\n", base64.StdEncoding.EncodeToString(info.RunningHash))
	fmt.Printf("  SequenceNumber - %d\n", info.SequenceNumber)
	fmt.Printf("  ExpirationTIme - %s\n", info.ExpirationTime)
	fmt.Printf("  AdminKey - %s\n", info.AdminKey)
	fmt.Printf("  SubmitKey - %s\n", info.SubmitKey)
	fmt.Printf("  AutoRenewPeriod - %s\n", info.AutoRenewPeriod)
	fmt.Printf("  AutoRenewAccount - %s\n", info.AutoRenewAccountID)

	return nil
}

func subscribeTopic(cmd *cobra.Command, args []string) error {
	topicID, err := hedera.TopicIDFromString(args[0])
	if err != nil {
		return fmt.Errorf("invalid topic ID: %s", err)
	}

	// default to 20
	count := 20
	if len(args) == 2 {
		count, err = strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid count: %s", err)
		}
		if count < 1 {
			return fmt.Errorf("invalid value for count: %d", count)
		}
	}

	mc, err := hedera.NewMirrorClient(config.mirrorNodeAddress)
	if err != nil {
		return fmt.Errorf("failed to create mirror client: %s", err)
	}

	fmt.Println("trying to subscribe to topic", topicID)

	respChan := make(chan *hedera.MirrorConsensusTopicResponse)
	infoChan := make(chan *hedera.ConsensusTopicInfo)
	errChan := make(chan error)
	handle, err := hedera.NewMirrorConsensusTopicQuery().
		SetTopicID(topicID).
		SetStartTime(unixEpoch).
		SetLimit(uint64(count)).
		Subscribe(mc,
			func(resp hedera.MirrorConsensusTopicResponse) {
				respChan <- &resp
			},
			func(err error) {
				errChan <- err
			},
		)

	defer func() {
		handle.Unsubscribe()
		mc.Close()
	}()

	received := 0
	lastSequenceNumber := uint64(0)
	ticker := time.NewTicker(500 * time.Millisecond)
	client := createClient()

	for {
		done := false

		select {
		case resp := <-respChan:
			fmt.Printf("received message (%d) of %d bytes payload, with sequence: %d\n", received, len(resp.Message), resp.SequenceNumber)
			received++
		case info := <-infoChan:
			if lastSequenceNumber != info.SequenceNumber {
				fmt.Println("topic info: last sequence number", info.SequenceNumber)
				lastSequenceNumber = info.SequenceNumber
			}
		case <-ticker.C:
			go func() {
				info, err := hedera.NewConsensusTopicInfoQuery().SetTopicID(topicID).Execute(client)
				if err != nil {
					logger.Errorf("failed to query topic info: %s", err)
				} else {
					infoChan <- &info
				}
			}()
		case err := <-errChan:
			logger.Errorf("error received during subscription: %s", err)
			done = true
			break
		}

		if done {
			ticker.Stop()
			break
		}
	}

	return nil
}

type benchmarkMessage struct {
	Magic     []byte
	Timestamp time.Time
	Payload   string
}

type messageWithTimestamp struct {
	message   []byte
	timestamp time.Time
}

func benchmarkTopic(cmd *cobra.Command, args []string) error {
	topicID, err := hedera.TopicIDFromString(args[0])
	if err != nil {
		return fmt.Errorf("invalid topic ID: %s", err)
	}

	mc, err := hedera.NewMirrorClient(config.mirrorNodeAddress)
	if err != nil {
		return fmt.Errorf("failed to create mirror client: %s", err)
	}
	defer mc.Close()

	// default to 20
	count := 20
	if len(args) == 2 {
		count, err = strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid count: %s", err)
		}
		if count < 1 {
			return fmt.Errorf("invalid value for count: %d", count)
		}
	}

	magic := make([]byte, 4)
	rand.Seed(time.Now().UnixNano())
	rand.Read(magic)

	msgChan := make(chan *messageWithTimestamp)
	errChan := make(chan error)
	handle, err := hedera.NewMirrorConsensusTopicQuery().
		SetTopicID(topicID).
		SetStartTime(unixEpoch).
		Subscribe(mc,
			func(resp hedera.MirrorConsensusTopicResponse) {
				message := messageWithTimestamp{
					message: resp.Message,
					timestamp: time.Now(),
				}
				msgChan <- &message
			},
			func(err error) {
				errChan <- err
			})
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %s", err)
	}

	ticker := time.NewTicker(500 * time.Millisecond) // a 500ms interval ticker
	produced := 0
	verified := 0
	rtts := make([]time.Duration, 0, 20)
	totalRtt := time.Duration(0)
	client := createClient()

	for {
		done := false

		select {
		case msgWithTs := <-msgChan:
			message := &benchmarkMessage{}
			if json.Unmarshal(msgWithTs.message, message) != nil {
				fmt.Println("failed to unmarshal the message, skip it...")
				continue
			}
			if bytes.Compare(message.Magic, magic) == 0 {
				rtt := msgWithTs.timestamp.Sub(message.Timestamp)
				fmt.Printf("roundtrip delay for message '%s' is %s\n", message.Payload, rtt)
				rtts = append(rtts, rtt)
				totalRtt += rtt
				verified++
			} else {
				fmt.Println("received a message not sent by us, skip it...")
			}
		case now := <-ticker.C:
			message := &benchmarkMessage{
				Magic:     magic,
				Timestamp: now,
				Payload:   fmt.Sprintf("hello %d", produced),
			}
			data, err := json.Marshal(message)
			if err != nil {
				logger.Fatalf("failed to marshal benchmarkMessage to JSON: %s", err)
			}
			_, err = hedera.NewConsensusMessageSubmitTransaction().
				SetTopicID(topicID).
				SetMessage(data).
				Execute(client)
			if err != nil {
				logger.Errorf("failed to submit consensus message: %s", err)
				continue
			}
			fmt.Printf("message '%s' sent\n", message.Payload)
			produced++
			if produced == count {
				ticker.Stop()
			}
		case err := <-errChan:
			logger.Errorf("error received during subscription: %s", err)
			done = true
		}

		if done {
			break
		}

		if verified == count {
			fmt.Printf("received all %d benchmark messages\n", count)
			handle.Unsubscribe()
			<-errChan
			break
		}
	}

	fmt.Printf("%d messages sent, %d message received\n", produced, verified)
	sort.Slice(rtts, func(i, j int) bool { return rtts[i] < rtts[j] })
	fmt.Printf("stats: average - %s, min - %s, max - %s, median - %s\n", totalRtt/time.Duration(verified), rtts[0], rtts[verified/2], rtts[verified-1])

	return nil
}

func makePublicKey(threshold uint32, keys []string, operatorPublicKey hedera.Ed25519PublicKey, exclOperator bool) (hedera.PublicKey, error) {
	if threshold < 1 {
		return nil, fmt.Errorf("invalid threshold: %d", threshold)
	}

	keySet := make(map[string]int)
	if keys != nil {
		for _, key := range keys {
			keySet[key] = 1
		}
	}
	if !exclOperator {
		keySet[operatorPublicKey.String()] = 1
	} else {
		delete(keySet, operatorPublicKey.String())
	}
	if len(keySet) == 0 {
		return nil, nil
	}
	if threshold > uint32(len(keySet)) {
		return nil, fmt.Errorf("threshold (%d) > number of keys (%d)", threshold, len(keySet))
	}

	thresholdKey := hedera.NewThresholdKey(threshold)
	var lastKey hedera.PublicKey
	for str := range keySet {
		publicKey, err := hedera.Ed25519PublicKeyFromString(str)
		if err != nil {
			return nil, err
		}
		thresholdKey.Add(&publicKey)
		lastKey = &publicKey
	}
	if len(keySet) == 1 {
		return lastKey, nil
	}
	return thresholdKey, nil
}
