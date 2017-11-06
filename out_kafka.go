// Copyright © 2017 Samsung CNCT
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Adapted from https://github.com/fluent/fluent-bit/blob/master/GOLANG_OUTPUT_PLUGIN.md

package main

import (
	"C"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"
	"unsafe"

	"github.com/Shopify/sarama"
	"github.com/fluent/fluent-bit-go/output"
	"github.com/ugorji/go/codec"
)

var brokerList = []string{"kafka-0.kafka.default.svc.cluster.local:9092"}
var producer sarama.SyncProducer
var timeout = 0 * time.Minute

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "out_kafka", "out_kafka GO!")
}

//export FLBPluginInit
func FLBPluginInit(ctx unsafe.Pointer) int {
	var err error

	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	// If Kafka is not running on init, wait to connect
	deadline := time.Now().Add(timeout)
	for tries := 0; time.Now().Before(deadline); tries++ {
		if producer == nil {
			producer, err = sarama.NewSyncProducer(brokerList, nil)
		}
		if err == nil {
			return output.FLB_OK
		}
		log.Printf("Cannot connect to Kafka: (%s) retrying...", err)
		time.Sleep(time.Second * 30)
	}
	log.Printf("Kafka failed to respond after %s", timeout)
	return output.FLB_ERROR
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {

	var ret int
	var ts interface{}
	var err error
	var record map[interface{}]interface{}
	var encData []byte

	dec := output.NewDecoder(data, int(length))

	// Iterate the original MessagePack array
	for {
		// Extract Record
		ret, ts, record = output.GetRecord(dec)
		if ret == 0 {
			break
		}
	}

	// select format until config files are available for fluentbit
	format := "json"

	switch format {
	case "json":
		encData, err = encodeAsJson(ts, record)
	case "msgpack":
		encData, err = encodeAsMsgpack(ts)
	case "string":
		// encData, err == encode_as_string(m)
	}

	if err != nil {
		fmt.Printf("Failed to encode %s data: %v\n", format, err)
		return output.FLB_ERROR
	}

	producer.SendMessage(&sarama.ProducerMessage{
		Topic: "logs_default",
		Key:   nil,
		Value: sarama.ByteEncoder(encData),
	})

	return output.FLB_OK
}

func encodeAsJson(ts interface{}, record map[interface{}]interface{}) ([]byte, error) {
	timestamp := ts.(output.FLBTime)

	type Log struct {
		Time   output.FLBTime
		Record interface{}
	}

	log := Log{
		Time:   timestamp,
		Record: record,
	}

	return json.Marshal(log)
}

func encodeAsMsgpack(ts interface{}) ([]byte, error) {
	var (
		mh codec.MsgpackHandle
		w  io.Writer
		b  []byte
	)

	enc := codec.NewEncoder(w, &mh)
	enc = codec.NewEncoderBytes(&b, &mh)
	err := enc.Encode(&ts)
	return b, err
}

func FLBPluginExit() int {
	return 0
}

func main() {
}
