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
func dataInsert(dataType pb.ECGPacket_DataType, deviceID string, dataPoint float64, timestamp uint64) {
	_, haveIt := redisClient.Info(deviceID)
	if haveIt != nil {
		redisClient.CreateKeyWithOptions(deviceID, redistimeseries.DefaultCreateOptions)
		redisClient.CreateKeyWithOptions(deviceID+"_avg", redistimeseries.DefaultCreateOptions)
		redisClient.CreateKeyWithOptions(deviceID+"_temp", redistimeseries.DefaultCreateOptions)
		redisClient.CreateRule(deviceID, redistimeseries.AvgAggregation, 60, deviceID+"_avg")
	}

	if dataType == pb.ECGPacket_RRI {
		_, err := redisClient.Add(deviceID, int64(timestamp), dataPoint)
		if err != nil {
			fmt.Println("Error:", err)
		}
	} else {
		//right now the default alternative is temp
		_, err := redisClient.Add(deviceID+"_temp", int64(timestamp), dataPoint)
		if err != nil {
			fmt.Println("Error:", err)
		}
	}

	fmt.Println("Insert data successfully.")
}

type JsonData struct {
	dp []redistimeseries.DataPoint
}

func reverseDataPoint(s []redistimeseries.DataPoint) []redistimeseries.DataPoint {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func dataQuery(datatype string, deviceID string, endTime int64, count int64) []redistimeseries.DataPoint {
	var ecgOptions = redistimeseries.RangeOptions{
		AggType:    "",
		TimeBucket: -1,
		Count:      count,
	}
	var dataPoints []redistimeseries.DataPoint
	if datatype == "ECG" {
		dataPoints, _ = redisClient.ReverseRangeWithOptions(deviceID, 0, endTime, ecgOptions)
	} else {
		dataPoints, _ = redisClient.ReverseRangeWithOptions(deviceID+"_temp", 0, endTime, ecgOptions)
	}

	return reverseDataPoint(dataPoints)
}

func corsHeaderSet(w http.ResponseWriter) {
	//FIXME: CORS allow all is not secure enough
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func parseQueryURL(req *http.Request) (string, int64, int64) {
	deviceID := ""
	var endTime, count int64
	count = 100
	for k, v := range req.URL.Query() {
		if k == "deviceId" {
			deviceID = v[0]
			continue
		} else if k == "endTime" {
			endTime, _ = strconv.ParseInt(v[0], 10, 64)
			continue
		} else if k == "count" {
			count, _ = strconv.ParseInt(v[0], 10, 64)
			continue
		}
	}
	return deviceID, endTime, count
}

func writeBackJsonPayload(w http.ResponseWriter, data []redistimeseries.DataPoint) {
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

var tempHttpQueryHandler = func(w http.ResponseWriter, req *http.Request) {
	deviceID, endTime, count := parseQueryURL(req)

	corsHeaderSet(w)

	data := dataQuery("TEMP", deviceID, endTime, count)

	writeBackJsonPayload(w, data)
}

var ecgHttpQueryHandler = func(w http.ResponseWriter, req *http.Request) {
	deviceID, endTime, count := parseQueryURL(req)

	corsHeaderSet(w)

	data := dataQuery("ECG", deviceID, endTime, count)

	writeBackJsonPayload(w, data)
}

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	packet := &pb.ECGPacket{}
	if err := proto.Unmarshal(msg.Payload(), packet); err != nil {
		log.Fatalln("Failed to parse address book:", err)
	}

	dataInsert(packet.DataType, packet.DeviceId, float64(packet.Value), packet.Time)
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

	// http://0.0.0.0:8888/ecg?deviceId=ED5A782825AB&endTime=1646945822002
	http.HandleFunc("/RRI", ecgHttpQueryHandler)

	http.HandleFunc("/TEMP", tempHttpQueryHandler)

	log.Println("Listing for requests at http://0.0.0.0:8000/RRI")
	log.Println("Listing for requests at http://0.0.0.0:8000/TEMP")

	log.Fatal(http.ListenAndServe(":8000", nil))
}
