package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/hashgraph/hedera-sdk-go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/alecthomas/kingpin.v2"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"
)

var (
	version = "v0.2.0"
	logger  *log.Logger

	configFile = kingpin.Flag("config", "json configuration file").Default("hedera_env.json").String()

	// account commands
	account           = kingpin.Command("account", "account operations")
	getBalance        = account.Command("balance", "get account balance")
	getBalanceAccount = getBalance.Arg("accountId", "account to query balance for, default to current account").String()

	// topic commands
	topic                         = kingpin.Command("topic", "topic operations")
	createTopic                   = topic.Command("create", "create a new HCS topic")
	createTopicCount              = createTopic.Arg("count", "the number of topics to create").Default("1").Int()
	createTopicMemo               = createTopic.Flag("memo", "memo for the topic").String()
	createTopicAdminKeys          = createTopic.Flag("admin-key", "public key of admin").Strings()
	createTopicAdminExclude       = createTopic.Flag("admin-exclude", "exclude self from admin").Bool()
	createTopicAdminKeyThreshold  = createTopic.Flag("admin-key-threshold", "threshold of admin key").Default("1").Uint32()
	createTopicSubmitKeys         = createTopic.Flag("submit-key", "public key allowed to submit messages").Strings()
	createTopicSubmitExclude      = createTopic.Flag("submit-exclude", "exclude self from submitting messages").Bool()
	createTopicSubmitKeyThreshold = createTopic.Flag("submit-key-threshold", "threshold of submit key").Default("1").Uint32()
	deleteTopic                   = topic.Command("delete", "delete the specified HCS topic")
	deleteTopicIds                = deleteTopic.Arg("id", "the topic id").Required().Strings()
	queryTopic                    = topic.Command("query", "query info of an HCS topic")
	queryTopicId                  = queryTopic.Arg("id", "the topic id").Required().String()
	subscribeTopic                = topic.Command("subscribe", "subscribe to a topic and show messages")
	subscribeTopicId              = subscribeTopic.Arg("id", "the topic id").Required().String()
	subscriptTopicMsgCount        = subscribeTopic.Arg("count", "number of messages to show").Default("20").Int()
	benchmarkTopic                = topic.Command("benchmark", "benchmark a topic")
	benchmarkTopicId              = benchmarkTopic.Arg("id", "the topic id").Required().String()

	// key commands
	key                 = kingpin.Command("key", "key operations")
	_                   = key.Command("gen", "generate ed25519 private key")
	getPubKey           = key.Command("pub", "get public key from ed25519 private key")
	getPubKeyPrivateKey = getPubKey.Arg("privkey", "the ed25519 private key string").Required().String()

	_ = kingpin.Command("version", "show version information")

	// global configs
	network            map[string]hedera.AccountID
	operatorId         hedera.AccountID
	operatorPrivateKey hedera.Ed25519PrivateKey
	mirrorNodeAddress  string

	unixEpoch = time.Unix(0, 0)
)

type HederaConfig struct {
	OperatorId        string
	OperatorKey       string
	NodeId            string // deprecated, replaced by Network
	NodeAddress       string // deprecated, replaced by Network
	MirrorNodeAddress string
}

func loadConfig(configFile string) error {
	v := viper.NewWithOptions(viper.KeyDelimiter("::"))
	v.AutomaticEnv()
	v.SetConfigName(configFile)
	v.SetConfigType("json")
	v.AddConfigPath(".")

	var err error
	if err = v.ReadInConfig(); err != nil {
		logger.Fatalf("failed to read configuration = %v", err)
	}
	var config HederaConfig
	if err := v.Unmarshal(&config); err != nil {
		logger.Fatalf("failed to unmarshal configuration = %v", err)
	}

	if operatorId, err = hedera.AccountIDFromString(config.OperatorId); err != nil {
		return fmt.Errorf("invalid node id = %v", err)
	}
	if operatorPrivateKey, err = hedera.Ed25519PrivateKeyFromString(config.OperatorKey); err != nil {
		return fmt.Errorf("invalid operator private key = %v", err)
	}
	mirrorNodeAddress = config.MirrorNodeAddress

	network = make(map[string]hedera.AccountID)
	if config.NodeAddress != "" && config.NodeId != "" {
		nodeId, err := hedera.AccountIDFromString(config.NodeId)
		if err != nil {
			return fmt.Errorf("invalid node id =  %v", err)
		}
		network[config.NodeAddress] = nodeId
	} else {
		// workaround for the viper issue that json string map in env var will not be converted to map[string]string
		networkVal := v.Get("network")
		var networkMap map[string]string
		switch val := networkVal.(type) {
		case string:
			networkMap = v.GetStringMapString("network")
		case map[string]interface{}:
			networkMap = v.GetStringMapString("network")
		default:
			return fmt.Errorf("invalid type of network - %T", val)
		}
		if len(networkMap) == 0 {
			return fmt.Errorf("empty network")
		}
		for address, id := range networkMap {
			nodeId, err := hedera.AccountIDFromString(id)
			if err != nil {
				return fmt.Errorf("invalid node id = %v", err)
			}
			network[address] = nodeId
		}
	}
	return nil
}

func createClient() *hedera.Client {
	client := hedera.NewClient(network)
	client.SetOperator(operatorId, operatorPrivateKey)
	return client
}

func doAccountBalance() {
	client := createClient()

	accountId := operatorId
	account := *getBalanceAccount
	if account != "" {
		tmpId, err := hedera.AccountIDFromString(account)
		if err != nil {
			logger.Fatalf("invalid accountId - %v", err)
		}
		accountId = tmpId
	}
	balance, err := hedera.NewAccountBalanceQuery().
		SetAccountID(accountId).
		Execute(client)
	if err != nil {
		logger.Fatalf("NewAccountBalanceQuery failed - %v", err)
	}
	logger.Infof("balance for account %s = %v", accountId, balance)
}

func doAccountCommand(cmd string) {
	switch cmd {
	case "balance":
		doAccountBalance()
	}
}

func makePublicKey(threshold uint32, keys []string, operatorPublicKey hedera.Ed25519PublicKey, exclOperator bool) (hedera.PublicKey, error) {
	if threshold < 1 {
		return nil, fmt.Errorf("invalid threshold - %d", threshold)
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

func doTopicCreate(client *hedera.Client) {
	count := *createTopicCount
	if count < 1 || count > 10 {
		logger.Fatalf("invalid count - %d\n", count)
	}

	operatorPublicKey := operatorPrivateKey.PublicKey()
	adminKey, err := makePublicKey(*createTopicAdminKeyThreshold, *createTopicAdminKeys, operatorPublicKey, *createTopicAdminExclude)
	if err != nil {
		logger.Fatalf("failed to create AdminKey for new topic - %v", err)
	}
	submitKey, err := makePublicKey(*createTopicSubmitKeyThreshold, *createTopicSubmitKeys, operatorPublicKey, *createTopicSubmitExclude)
	if err != nil {
		logger.Fatalf("failed to create SubmitKey for new topic - %v", err)
	}
	for i := 0; i < count; i++ {
		tx := hedera.NewConsensusTopicCreateTransaction()
		if *createTopicMemo != "" {
			tx.SetTopicMemo(*createTopicMemo)
		}
		if adminKey != nil {
			tx.SetAdminKey(adminKey)
		}
		if submitKey != nil {
			tx.SetSubmitKey(submitKey)
		}
		txId, err := tx.Execute(client)
		if err != nil {
			logger.Fatalf("failed to create a new topic, %v", err)
		}
		receipt, err := txId.GetReceipt(client)
		if err != nil {
			logger.Fatalf("failed to get receipt of topic create transaction, %v", err)
		}
		logger.Infoln("new topic id:", receipt.GetConsensusTopicID())
	}
}

func doTopicDelete(client *hedera.Client) {
	for _, topic := range *deleteTopicIds {
		topicId, err := hedera.TopicIDFromString(topic)
		if err != nil {
			logger.Errorf("invalid topic id %s, err = %v", topic, err)
			continue
		}

		txId, err := hedera.NewConsensusTopicDeleteTransaction().
			SetMemo("hcscli tool, delete topic").
			SetTopicID(topicId).
			Execute(client)
		if err != nil {
			logger.Errorf("failed to delete topic %s = %v", topic, err)
			continue
		}

		_, err = txId.GetReceipt(client)
		if err != nil {
			logger.Infof("failed to get receipt for topic %s deletion = %v", topic, err)
		} else {
			logger.Infof("topic %s is deleted", topic)
		}
	}
}

func doTopicInfoQuery(client *hedera.Client) {
	topicId, err := hedera.TopicIDFromString(*queryTopicId)
	if err != nil {
		logger.Fatalf("invalid topic id %s, err = %v", *queryTopicId, err)
	}
	info, err := hedera.NewConsensusTopicInfoQuery().
		SetTopicID(topicId).
		Execute(client)
	if err != nil {
		logger.Fatalf("failed to query topic (%s) info = %v", *queryTopicId, err)
	}

	logger.Infof("Info of topic %s:", *queryTopicId)
	logger.Infof("\tMemo: %s", info.Memo)
	logger.Infof("\tSequencenumber: %d", info.SequenceNumber)
	logger.Infof("\tExpirationTime: %v", info.ExpirationTime)
	logger.Infof("\tAutoRenewPeriod: %v", info.AutoRenewPeriod)
	logger.Infof("\tAutoRenewAccountID: %v", info.AutoRenewAccountID)
}

type subResp struct {
	resp *hedera.MirrorConsensusTopicResponse
	info *hedera.ConsensusTopicInfo
}

func doTopicSubscribe(client *hedera.Client) {
	topicId, err := hedera.TopicIDFromString(*subscribeTopicId)
	if err != nil {
		logger.Fatalf("invalid topic id %s, err = %v", *subscribeTopicId, err)
	}

	mirrorClient, err := hedera.NewMirrorClient(mirrorNodeAddress)
	if err != nil {
		logger.Fatalf("failed to create mirror client = %v", err)
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
				logger.Infof("mirror subscription error = %v", err)
			})
	if err != nil {
		logger.Fatalf("failed to create mirror consensus topic query = %v", err)
	}

	defer func() {
		handle.Unsubscribe()
		logger.Info("unsubscribed from topic, sleep 1 second")
		time.Sleep(time.Second)
	}()

	lastSequenceNumber := ^uint64(0)
	for {
		select {
		case res := <-respCh:
			if res.resp != nil {
				logger.Infof("received message of %d bytes payload, with sequence: %d", len(res.resp.Message), res.resp.SequenceNumber)
				count++
			} else {
				if lastSequenceNumber != res.info.SequenceNumber {
					logger.Infoln("topic info: last sequence number", res.info.SequenceNumber)
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

type consensusResponseMetadata struct {
	message   []byte
	timestamp time.Time
}

type benchmarkMessage struct {
	Magic     []byte
	Timestamp time.Time
	Payload   string
}

func doTopicBenchmark(client *hedera.Client) {
	topicId, err := hedera.TopicIDFromString(*benchmarkTopicId)
	if err != nil {
		logger.Fatalf("invalid topic id %s, err = %v", *benchmarkTopicId, err)
	}

	mirrorClient, err := hedera.NewMirrorClient(mirrorNodeAddress)
	if err != nil {
		logger.Fatalf("failed to create mirror client = %v", err)
	}

	magic := make([]byte, 4)
	rand.Seed(time.Now().UnixNano())
	rand.Read(magic)

	ch := make(chan *consensusResponseMetadata)
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
					metadata := consensusResponseMetadata{
						message:   resp.Message,
						timestamp: time.Now(),
					}
					ch <- &metadata
				}
			},
			func(err error) {
				logger.Errorf("mirror subscription error = %v", err)
			})
	if err != nil {
		logger.Fatalf("failed to create mirror consensus topic query = %v", err)
	}

	defer func() {
		handle.Unsubscribe()
		logger.Print("unsubscribed from topic, sleep 1 second")
		time.Sleep(time.Second)
	}()

	ticker := time.NewTicker(500 * time.Millisecond) // a 500ms interval ticker
	produced := 0
	verified := 0
	rtts := make([]time.Duration, 0, 20)
	totalRtt := time.Duration(0)

	for {
		select {
		case metadata := <-ch:
			message := &benchmarkMessage{}
			if json.Unmarshal(metadata.message, message) != nil {
				logger.Info("failed to unmarshal the message, skip it...")
				continue
			}
			if bytes.Compare(message.Magic, magic) == 0 {
				rtt := metadata.timestamp.Sub(message.Timestamp)
				logger.Infof("roundtrip delay for message '%s' is %v", message.Payload, rtt)
				rtts = append(rtts, rtt)
				totalRtt += rtt
				verified++
			} else {
				logger.Infoln("received a message not sent by us, skip it...")
			}
		case now := <-ticker.C:
			message := &benchmarkMessage{
				Magic:     magic,
				Timestamp: now,
				Payload:   fmt.Sprintf("hello %d", produced),
			}
			data, err := json.Marshal(message)
			if err != nil {
				logger.Fatalf("failed to marshal benchmarkMessage to JSON = %v", err)
			}
			_, err = hedera.NewConsensusMessageSubmitTransaction().
				SetTopicID(topicId).
				SetMessage(data).
				Execute(client)
			if err != nil {
				logger.Fatalf("failed to submit consensus message = %v", err)
			}
			logger.Infof("message '%s' sent", message.Payload)
			produced++
			if produced == 20 {
				ticker.Stop()
			}
		}

		if verified == 20 {
			logger.Infoln("received all 20 benchmark messages")
			close(done)
			break
		}
	}
	sort.Slice(rtts, func(i, j int) bool { return rtts[i] < rtts[j] })
	logger.Infof("stats: average - %v, min - %v, max - %v, median - %v", totalRtt/20, rtts[0], rtts[19], rtts[10])
}

func doTopicCommand(cmd string) {
	client := createClient()
	switch cmd {
	case "create":
		doTopicCreate(client)
	case "delete":
		doTopicDelete(client)
	case "query":
		doTopicInfoQuery(client)
	case "subscribe":
		doTopicSubscribe(client)
	case "benchmark":
		doTopicBenchmark(client)
	}
}

func doKeyCommand(cmd string) {
	switch cmd {
	case "gen":
		// generate ed25519 private key and print it
		if privKey, err := hedera.GenerateEd25519PrivateKey(); err != nil {
			panic(err)
		} else {
			logger.Infoln("generated ed25519 private key, please save it in a secure store:", privKey)
			logger.Infoln("ed25519 public key:", privKey.PublicKey())
		}
	case "pub":
		// get public key
		if privKey, err := hedera.Ed25519PrivateKeyFromString(*getPubKeyPrivateKey); err != nil {
			logger.Fatalf("invalid private key - %v", err)
		} else {
			logger.Infoln("ed25519 public key:", privKey.PublicKey())
		}
	}
}

func createLogger() {
	logger = log.New()
	logger.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	logger.SetOutput(os.Stdout)
}

func main() {
	createLogger()
	// parse the command line
	cmds := strings.Split(kingpin.Parse(), " ")
	if cmds[0] == "version" {
		logger.Infoln("version:", version)
		return
	}

	if err := loadConfig(*configFile); err != nil {
		logger.Fatalf("error loading config = %v", err)
	}

	switch cmd := cmds[0]; cmd {
	case "account":
		doAccountCommand(cmds[1])
	case "topic":
		doTopicCommand(cmds[1])
	case "key":
		doKeyCommand(cmds[1])
	}
}
