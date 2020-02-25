package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/hashgraph/hedera-sdk-go"
	"gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"
)

var (
	configFile = kingpin.Flag("config", "json configuration file").Default("./hedera_env.json").String()

	// account commands
	account           = kingpin.Command("account", "account operations")
	getBalance        = account.Command("balance", "get account balance")
	getBalanceAccount = getBalance.Arg("accountId", "account to query balance for, default to current account").String()

	// topic commands
	topic                  = kingpin.Command("topic", "topic operations")
	createTopic            = topic.Command("create", "create a new HCS topic")
	createTopicCount       = createTopic.Arg("count", "the number of topics to create").Default("1").Int()
	deleteTopic            = topic.Command("delete", "delete the specified HCS topic")
	deleteTopicIds         = deleteTopic.Arg("id", "the topic id").Required().Strings()
	queryTopic             = topic.Command("query", "query info of an HCS topic")
	queryTopicId           = queryTopic.Arg("id", "the topic id").Required().String()
	subscribeTopic         = topic.Command("subscribe", "subscribe to a topic and show messages")
	subscribeTopicId       = subscribeTopic.Arg("id", "the topic id").Required().String()
	subscriptTopicMsgCount = subscribeTopic.Arg("count", "number of messages to show").Default("20").Int()
	benchmarkTopic         = topic.Command("benchmark", "benchmark a topic")
	benchmarkTopicId       = benchmarkTopic.Arg("id", "the topic id").Required().String()

	// key commands
	key                 = kingpin.Command("key", "key operations")
	_                   = key.Command("gen", "generate ed25519 private key")
	getPubKey           = key.Command("pub", "get public key from ed25519 private key")
	getPubKeyPrivateKey = getPubKey.Arg("privkey", "the ed25519 private key string").Required().String()

	// global configs
	nodeId             hedera.AccountID
	nodeAddress        string
	operatorId         hedera.AccountID
	operatorPrivateKey hedera.Ed25519PrivateKey
	mirrorNodeAddress  string

	unixEpoch = time.Unix(0, 0)
)

type HederaConfig struct {
	OperatorId        string
	OperatorKey       string
	NodeId            string
	NodeAddress       string
	MirrorNodeAddress string
}

func loadConfigFromFile(filename string) (*HederaConfig, error) {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	config := HederaConfig{}
	err = json.Unmarshal([]byte(file), &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func loadConfigFromEnv() (*HederaConfig, error) {
	config := HederaConfig{}
	var allValues = []struct {
		value *string
		name  string
	}{
		{&config.OperatorId, "OPERATOR_ID"},
		{&config.OperatorKey, "OPERATOR_KEY"},
		{&config.NodeId, "NODE_ID"},
		{&config.NodeAddress, "NODE_ADDRESS"},
		{&config.MirrorNodeAddress, "MIRROR_NODE_ADDRESS"},
	}

	for _, entry := range allValues {
		*entry.value = os.Getenv(entry.name)
		if *entry.value == "" {
			return nil, fmt.Errorf("env variable %s is not set", entry.name)
		}
	}

	return &config, nil
}

func loadConfig() error {
	configSource := fmt.Sprintf("file - %s", *configFile)
	config, err := loadConfigFromFile(*configFile)
	if err != nil {
		configSource = "environment variables"
		if config, err = loadConfigFromEnv(); err != nil {
			return err
		}
	}

	// parse the config
	if nodeId, err = hedera.AccountIDFromString(config.NodeId); err != nil {
		return fmt.Errorf("invalid node id string from %s = %v", configSource, err)
	}
	if operatorId, err = hedera.AccountIDFromString(config.OperatorId); err != nil {
		return fmt.Errorf("invalid operator id string from %s = %v", configSource, err)
	}
	if operatorPrivateKey, err = hedera.Ed25519PrivateKeyFromString(config.OperatorKey); err != nil {
		return fmt.Errorf("invalid operator private key string from %s = %v", configSource, err)
	}
	nodeAddress = config.NodeAddress
	mirrorNodeAddress = config.MirrorNodeAddress
	fmt.Printf("loaded configuration from %s\n", configSource)
	return nil
}

func createClient() *hedera.Client {
	nodes := map[string]hedera.AccountID{nodeAddress: nodeId}
	client := hedera.NewClient(nodes)
	client.SetOperator(operatorId, operatorPrivateKey)
	return client
}

func doAccountBalance(account string) {
	client := createClient()

	var err error
	accountId := operatorId
	if account != "" {
		accountId, err = hedera.AccountIDFromString(account)
		if err != nil {
			panic(err)
		}
	}
	var balance hedera.Hbar
	balance, err = hedera.NewAccountBalanceQuery().
		SetAccountID(accountId).
		Execute(client)
	if err != nil {
		panic(err)
	}

	fmt.Printf("balance for account %v = %v\n", accountId, balance)
}

func doAccountCommand(cmd string) {
	switch cmd {
	case "balance":
		doAccountBalance(*getBalanceAccount)
	default:
		fmt.Println("unknown command", cmd)
	}
}

func doTopicCreate(client *hedera.Client, count int) []*hedera.ConsensusTopicID {
	if count < 0 || count > 10 {
		fmt.Printf("You can't create %d topic(s)\n", count)
		return nil
	}

	operatorPublicKey := operatorPrivateKey.PublicKey()
	result := make([]*hedera.ConsensusTopicID, count)
	for i := 0; i < count; i++ {
		txId, err := hedera.NewConsensusTopicCreateTransaction().
			SetAdminKey(operatorPublicKey).
			SetTransactionMemo("hcscli tool, create topic").
			// SetMaxTransa	ctionFee(hedera.HbarFrom(8, hedera.HbarUnits.Hbar)).
			Execute(client)
		if err != nil {
			fmt.Printf("failed to create a new topic, %v", err)
			continue
		}

		receipt, err := txId.GetReceipt(client)
		if err != nil {
			fmt.Printf("failed to get receipt of topic create transaction, %v", err)
			continue
		}
		topicId := receipt.GetConsensusTopicID()
		result[i] = &topicId
	}

	return result
}

func doTopicDelete(client *hedera.Client, topics *[]string) {
	for _, topic := range *topics {
		topicId, err := hedera.TopicIDFromString(topic)
		if err != nil {
			fmt.Printf("invalid topic id %s, err = %v\n", topic, err)
			continue
		}

		txId, err := hedera.NewConsensusTopicDeleteTransaction().
			SetMemo("hcscli tool, delete topic").
			SetTopicID(topicId).
			Execute(client)
		if err != nil {
			fmt.Println("failed to delete topic", topic, err)
			continue
		}

		_, err = txId.GetReceipt(client)
		if err != nil {
			fmt.Println("failed to get receipt for topic deletion,", topic, err)
		} else {
			fmt.Printf("topic %s is deleted\n", topic)
		}
	}
}

func doTopicInfoQuery(client *hedera.Client, topicIdStr string) {
	topicId, err := hedera.TopicIDFromString(topicIdStr)
	if err != nil {
		fmt.Printf("invalid topic id %s, err = %v\n", topicIdStr, err)
		return
	}

	query := hedera.NewConsensusTopicInfoQuery()
	q := query.SetTopicID(topicId)

	info, err := q.Execute(client)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Info of topic %s:\n", topicIdStr)
	fmt.Printf("\tMemo: %s\n", info.Memo)
	fmt.Printf("\tSequencenumber: %d\n", info.SequenceNumber)
	fmt.Printf("\tExpirationTime: %v\n", info.ExpirationTime)
	fmt.Printf("\tAutoRenewPeriod: %v\n", info.AutoRenewPeriod)
	fmt.Printf("\tAutoRenewAccountID: %v\n", info.AutoRenewAccountID)
}

type subResp struct {
	resp *hedera.MirrorConsensusTopicResponse
	info *hedera.ConsensusTopicInfo
}

func doTopicSubscribe(client *hedera.Client, topicIdStr string) {
	topicId, err := hedera.TopicIDFromString(topicIdStr)
	if err != nil {
		fmt.Printf("invalid topic id %s, err = %v\n", topicIdStr, err)
		return
	}

	mirrorClient, err := hedera.NewMirrorClient(mirrorNodeAddress)
	if err != nil {
		fmt.Printf("err creating mirror client = %v\n", err)
		return
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	done := make(chan struct{})
	respCh := make(chan subResp)
	count := 0
	total := *subscriptTopicMsgCount
	handle, err := hedera.NewMirrorConsensusTopicQuery().
		SetTopicID(topicId).
		SetStartTime(unixEpoch).
		Subscribe(
			mirrorClient,
			func(resp hedera.MirrorConsensusTopicResponse) {
				select {
				case <-done:
					// do nothing when done
				default:
					res := subResp{
						resp: &resp,
						info: nil,
					}
					respCh <- res
				}
			},
			func(err error) {
				fmt.Println(err.Error())
			})
	if err != nil {
		panic(err)
	}

	defer func() {
		handle.Unsubscribe()
		fmt.Println("unsubscribed from topic, sleep 1 second")
		time.Sleep(time.Second)
	}()

	lastSequenceNumber := ^uint64(0)
	for {
		select {
		case res := <-respCh:
			if res.resp != nil {
				fmt.Printf("received message of %d bytes payload, with sequence: %d\n", len(res.resp.Message), res.resp.SequenceNumber)
				count++
			} else {
				if lastSequenceNumber != res.info.SequenceNumber {
					fmt.Printf("topic info: last sequence number %d\n", res.info.SequenceNumber)
					lastSequenceNumber = res.info.SequenceNumber
				}
			}
		case <-ticker.C:
			go func() {
				info, err := hedera.NewConsensusTopicInfoQuery().SetTopicID(topicId).Execute(client)
				if err != nil {
					panic(err)
				}
				res := subResp{
					resp: nil,
					info: &info,
				}
				respCh <- res
			}()
		}

		if count >= total {
			close(done)
			ticker.Stop()
			break
		}
	}
}

type ConsensusResponseMetadata struct {
	Message   []byte
	Timestamp time.Time
}

func doTopicBenchmark(client *hedera.Client, topicIdStr string) {
	topicId, err := hedera.TopicIDFromString(topicIdStr)
	if err != nil {
		fmt.Printf("invalid topic id %s, err = %v\n", topicIdStr, err)
		return
	}

	mirrorClient, err := hedera.NewMirrorClient(mirrorNodeAddress)
	if err != nil {
		panic(err)
	}

	magic := make([]byte, 4)
	rand.Seed(time.Now().UnixNano())
	rand.Read(magic)

	ch := make(chan *ConsensusResponseMetadata)
	done := make(chan struct{})
	handle, err := hedera.NewMirrorConsensusTopicQuery().
		SetTopicID(topicId).
		SetStartTime(unixEpoch).
		Subscribe(
			mirrorClient,
			func(resp hedera.MirrorConsensusTopicResponse) {
				select {
				case <-done:
					// do nothing
				default:
					metadata := ConsensusResponseMetadata{
						Message:   resp.Message,
						Timestamp: time.Now(),
					}
					ch <- &metadata
				}
			},
			func(err error) {
				fmt.Println(err.Error())
			})
	if err != nil {
		panic(err)
	}

	defer func() {
		handle.Unsubscribe()
		fmt.Println("unsubscribed from topic, sleep 1 second")
		time.Sleep(time.Second)
	}()

	stats := make(map[string]time.Time)
	ticker := time.NewTicker(500 * time.Millisecond) // a 500ms interval ticker
	produced := 0
	verified := 0

	for {
		select {
		case metadata := <-ch:
			data := metadata.Message
			if bytes.Compare(data[0:len(magic)], magic) == 0 {
				msg := string(data[len(magic):])
				start, ok := stats[msg]
				if !ok {
					fmt.Printf("cannot find message '%s' in stats map\n", msg)
				} else {
					fmt.Printf("roundtrip delay for message '%s' is %v\n", msg, metadata.Timestamp.Sub(start))
					delete(stats, msg)
					verified++
				}
			} else {
				fmt.Printf("recieved a messsage not sent by us, skip it...\n")
			}
		case start := <-ticker.C:
			msg := fmt.Sprintf("hello %d", produced)
			data := append(magic, []byte(msg)...)
			stats[msg] = start
			id, err := hedera.NewConsensusMessageSubmitTransaction().
				SetTopicID(topicId).
				SetMessage(data).
				Execute(client)
			if err != nil {
				panic(err)
			}

			_, err = id.GetReceipt(client)
			if err != nil {
				panic(err)
			}

			fmt.Printf("message '%s' sent\n", msg)
			produced++
			if produced == 20 {
				ticker.Stop()
			}
		}

		if verified == 20 {
			fmt.Printf("received all 20 benchmark messages\n")
			close(done)
			break
		}
	}
}

func doTopicCommand(cmd string) {
	client := createClient()

	switch cmd {
	case "create":
		topics := doTopicCreate(client, *createTopicCount)
		for _, topic := range topics {
			if topic != nil {
				fmt.Printf("new topic id: %s\n", *topic)
			}
		}
	case "delete":
		doTopicDelete(client, deleteTopicIds)
	case "query":
		doTopicInfoQuery(client, *queryTopicId)
	case "subscribe":
		doTopicSubscribe(client, *subscribeTopicId)
	case "benchmark":
		doTopicBenchmark(client, *benchmarkTopicId)
	default:
		fmt.Println("unknown command", cmd)
	}
}

func doKeyCommand(cmd string) {
	switch cmd {
	case "gen":
		// generate ed25519 private key and print it
		if privKey, err := hedera.GenerateEd25519PrivateKey(); err != nil {
			panic(err)
		} else {
			fmt.Printf("generated ed25519 private key, please save it in a secure store:\n%s\n", privKey.String())
			fmt.Printf("public key:\n%s\n", privKey.PublicKey().String())
		}
	case "pub":
		// get public key
		if privKey, err := hedera.Ed25519PrivateKeyFromString(*getPubKeyPrivateKey); err != nil {
			panic(err)
		} else {
			fmt.Printf("ed25519 public key:\n%s\n", privKey.PublicKey().String())
		}
	default:
		fmt.Println("unknown command", cmd)
	}
}

func main() {
	// parse the command line
	cmds := strings.Split(kingpin.Parse(), " ")

	if err := loadConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "error loading config = %v\n", err)
		os.Exit(1)
	}

	switch cmd := cmds[0]; cmd {
	case "account":
		doAccountCommand(cmds[1])
	case "topic":
		doTopicCommand(cmds[1])
	case "key":
		doKeyCommand(cmds[1])
	default:
		fmt.Printf("unknown command %s\n", cmd)
	}
}
