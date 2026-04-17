//go:build integration

package integration_test

import (
	cepheusstamp2 "cepheus/stamp"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var config cepheusstamp2.Config

func TestMain(m *testing.M) {
	config = cepheusstamp2.Config{
		ErrorEstimate: cepheusstamp2.ErrorEstimateConfig{
			Scale:        22,
			Multiplier:   1,
			Synchronized: true,
			ClockFormat:  cepheusstamp2.ClockFormatNTP,
		},
	}

	m.Run()
}

func Test_SendNormalPkt(t *testing.T) {
	t.Run("send normal packet", func(t *testing.T) {
		senderConfig := cepheusstamp2.SenderConfig{
			Config:     config,
			LocalAddr:  "localhost:50023",
			RemoteAddr: "localhost:862",
			HMACKey:    nil,
			Timeout:    5 * time.Second,
			OnError: func(err error) {
				t.Logf("error %v", err)
			},
		}
		sender, err := cepheusstamp2.NewSender(senderConfig)
		if err != nil {
			t.Fatalf("failed to create sender: %v", err)
		}

		response, err := sender.Send()
		if err != nil {
			t.Fatalf("failed to send packet: %v", err)
		}

		responseM, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("failed to marshal response: %v", err)
		}

		t.Logf("got response: %s", string(responseM))

		// timestamp sequence
		// session-sender timestamp < recieve-timestamp < timestamp RFC 8762§4.3.1
		timestamp, err1 := response.Timestamp.ToTime(senderConfig.Config.ErrorEstimate.ClockFormat)
		recieveTimestamp, err2 := response.ReceiveTimestamp.ToTime(sender.Config.ErrorEstimate.ClockFormat)
		senderTimestamp, err3 := response.SenderTimestamp.ToTime(sender.Config.ErrorEstimate.ClockFormat)

		if err1 != nil || err2 != nil || err3 != nil {
			log.Fatalf("failed to parse NTP timestamps to time.Time: %v %v %v", err1, err2, err3)
		}

		assert.Greater(t, *timestamp, *recieveTimestamp)
		assert.Greater(t, *recieveTimestamp, *senderTimestamp)
	})
}
