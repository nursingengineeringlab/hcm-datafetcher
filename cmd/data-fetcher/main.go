package main

import (
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	redistimeseries "github.com/RedisTimeSeries/redistimeseries-go"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	pb "github.com/shiywang/hcm-datafetcher/proto-gen/github.com/shiywang/hcm-datafetcher"
)

var redisClient *redistimeseries.Client

// https://github.com/RedisTimeSeries/RedisTimeSeries
// https://github.com/RedisTimeSeries/redistimeseries-go/
func dataInsert(deviceID string, dataPoint float64) {
	_, haveIt := redisClient.Info(deviceID)
	if haveIt != nil {
		redisClient.CreateKeyWithOptions(deviceID, redistimeseries.DefaultCreateOptions)
		redisClient.CreateKeyWithOptions(deviceID+"_avg", redistimeseries.DefaultCreateOptions)
		redisClient.CreateRule(deviceID, redistimeseries.AvgAggregation, 60, deviceID+"_avg")
	}
	// Add sample with timestamp from server time and value 100
	// TS.ADD mytest * 100
	_, err := redisClient.AddAutoTs(deviceID, dataPoint)
	if err != nil {
		fmt.Println("Error:", err)
	}
	fmt.Println("Insert data successfully.")
}

type JsonData struct {
	dp []redistimeseries.DataPoint
}

func dataQuery(deviceID string, endTime int64) []redistimeseries.DataPoint {
	var hour24 int64 = 86400000
	dataPoints, _ := redisClient.RangeWithOptions(deviceID, endTime-hour24, endTime, redistimeseries.DefaultRangeOptions)
	return dataPoints
}

var httpQueryHandler = func(w http.ResponseWriter, req *http.Request) {
	//io.WriteString(w, "Hello, world!\n")

	var deviceID string
	var endTime int64
	for k, v := range req.URL.Query() {
		fmt.Printf("%s: %s\n", k, v)
		if k == "deviceId" {
			deviceID = v[0]
			continue
		}
		if k == "endTime" {
			endTime, _ = strconv.ParseInt(v[0], 10, 64)
			continue
		}
	}

	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	type User struct {
		Id      string
		Balance uint64
	}
	//u := User{Id: "US123", Balance: 8}
	data := dataQuery(deviceID, endTime)

	js := JsonData{}
	counter := 1
	for _, v := range data {
		js.dp = append(js.dp, v)
		if counter == 500 {
			break
		}
		counter++
	}

	json.NewEncoder(w).Encode(js.dp)
}

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	//fmt.Printf("TOPIC: %s\n", msg.Topic())
	//fmt.Printf("MSG: %s\n", msg.Payload()).
	packet := &pb.ECGPacket{}
	if err := proto.Unmarshal(msg.Payload(), packet); err != nil {
		log.Fatalln("Failed to parse address book:", err)
	}
	fmt.Println(packet)
	if packet.DataType == pb.ECGPacket_RRI {
		dataInsert(packet.DeviceId, float64(packet.Value))
	}
}

func main() {
	// Hello world, the web server

	mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker("tcp://172.24.41.85:1883").SetClientID("emqx_data_fetcher")

	opts.SetKeepAlive(60 * time.Second)
	// Set the message callback handler
	opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	// Subscribe to a topic
	if token := c.Subscribe("emqtt", 0, nil); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	redisClient = redistimeseries.NewClient("172.24.41.85:6379", "nohelp", nil)

	// http://0.0.0.0:8888/graph?deviceId=ED5A782825AB&endTime=1646945822002
	http.HandleFunc("/graph", httpQueryHandler)
	log.Println("Listing for requests at http://0.0.0.0:8000/graph")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
