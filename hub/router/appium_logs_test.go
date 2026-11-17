/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"GADS/common/db"
	"GADS/common/models"
)

func appiumLog(timestamp, sequenceNumber int64) models.AppiumPluginLog {
	return models.AppiumPluginLog{Timestamp: timestamp, SequenceNumber: sequenceNumber, Message: "log"}
}

func appiumLogDoc(timestamp, sequenceNumber int64) bson.D {
	return bson.D{
		{Key: "message", Value: "log"},
		{Key: "timestamp", Value: timestamp},
		{Key: "sequenceNumber", Value: sequenceNumber},
	}
}

func TestAppiumLogTail(t *testing.T) {
	tail := newAppiumLogTail([]models.AppiumPluginLog{appiumLog(9000, 4), appiumLog(10000, 5), appiumLog(10000, 7)})
	assert.Equal(t, int64(8000), tail.since())

	// The poll reads the overlap again - the already sent logs are skipped,
	// the late stored (10000, 6) and the new one are returned in order
	// and (8500, 3) is older than the first event - history the client did not ask for
	logs := tail.unsent([]models.AppiumPluginLog{
		appiumLog(8500, 3),
		appiumLog(9000, 4),
		appiumLog(10000, 5),
		appiumLog(10000, 6),
		appiumLog(10000, 7),
		appiumLog(11500, 8),
	})
	assert.Equal(t, []models.AppiumPluginLog{appiumLog(10000, 6), appiumLog(11500, 8)}, logs)
	assert.Equal(t, int64(9500), tail.since())
	// (9000, 4) is out of the overlap now and is forgotten
	assert.Len(t, tail.sent, 4)

	// Nothing new or a failed read results in an empty array, not null
	logs = tail.unsent(nil)
	jsonData, err := json.Marshal(logs)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(jsonData))
}

func TestAppiumLogTailWithoutInitialLogs(t *testing.T) {
	tail := newAppiumLogTail([]models.AppiumPluginLog{})
	assert.Equal(t, -appiumLogStreamOverlap, tail.since())

	logs := tail.unsent([]models.AppiumPluginLog{appiumLog(1000, 0), appiumLog(1000, 1)})
	assert.Equal(t, []models.AppiumPluginLog{appiumLog(1000, 0), appiumLog(1000, 1)}, logs)
}

func TestAppiumLogsSSE(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("streams new logs after the latest ones", func(mt *mtest.T) {
		previousStore := db.GlobalMongoStore
		db.GlobalMongoStore = &db.MongoStore{Client: mt.Client, Ctx: context.Background()}
		defer func() { db.GlobalMongoStore = previousStore }()

		ns := "appium_logs_new.device1"
		mt.AddMockResponses(
			// Latest logs, newest first as GetAppiumLogs sorts them
			mtest.CreateCursorResponse(0, ns, mtest.FirstBatch,
				appiumLogDoc(2000, 3), appiumLogDoc(2000, 1), appiumLogDoc(1000, 0)),
			// First poll - (2000, 2) was stored after the first read
			mtest.CreateCursorResponse(0, ns, mtest.FirstBatch,
				appiumLogDoc(1000, 0), appiumLogDoc(2000, 1), appiumLogDoc(2000, 2), appiumLogDoc(2000, 3), appiumLogDoc(2100, 4)),
		)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/appium-logs/stream", AppiumLogsSSE)
		server := httptest.NewServer(router)
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/appium-logs/stream?collection=device1&logLimit=3", nil)
		require.NoError(mt, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(mt, err)
		defer resp.Body.Close()
		assert.Equal(mt, http.StatusOK, resp.StatusCode)
		assert.True(mt, strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream"))

		var events [][]models.AppiumPluginLog
		scanner := bufio.NewScanner(resp.Body)
		for len(events) < 2 && scanner.Scan() {
			data, ok := strings.CutPrefix(scanner.Text(), "data:")
			if !ok {
				continue
			}
			var logs []models.AppiumPluginLog
			require.NoError(mt, json.Unmarshal([]byte(data), &logs))
			events = append(events, logs)
		}
		require.Len(mt, events, 2)
		cancel()

		assert.Equal(mt, []models.AppiumPluginLog{appiumLog(1000, 0), appiumLog(2000, 1), appiumLog(2000, 3)}, events[0])
		assert.Equal(mt, []models.AppiumPluginLog{appiumLog(2000, 2), appiumLog(2100, 4)}, events[1])

		server.Close()
		started := mt.GetAllStartedEvents()
		require.GreaterOrEqual(mt, len(started), 2)
		assert.Equal(mt, "find", started[1].CommandName)
		filter := started[1].Command.Lookup("filter").Document()
		assert.Equal(mt, int64(0), filter.Lookup("timestamp", "$gte").Int64())
	})
}
