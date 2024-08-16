package controllers_test

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/go-numb/go-line-kintai/controllers"
	"github.com/stretchr/testify/assert"
)

const (
	TESTPROJECTID = ""
)

func TestToCsvByte(t *testing.T) {
	length := 10

	var rows []map[string]interface{}
	for i := 0; i < length; i++ {
		utc := time.Now().UTC()
		y, month, d := utc.Add(9 * time.Hour).Date()
		weekday := controllers.WeekdayToString(utc.Add(9 * time.Hour).Weekday())
		h, minites, _ := utc.Add(9 * time.Hour).Clock()
		rows = append(rows, map[string]interface{}{
			"year":    y,
			"month":   int(month),
			"day":     d,
			"weekday": weekday,
			"hour":    h,
			"minites": minites,

			"user_id":      "test",
			"display_name": fmt.Sprintf("name%d", i),
			"status":       "修正/備考",
			"note":         "test",
			"timestamp":    utc.UnixMilli(),
		})
	}

	data, err := controllers.ToCsvByte(rows)
	assert.NoError(t, err)

	log.Println(string(data))
}

func TestSaveToBacket(t *testing.T) {
	// make csv data
	data := []byte(`id,name,age,address,phone,email,created_at
1,name1,20,address1,090-1111-1111,test@gmail.com,2021-01-01`)

	// save to cloud storage
	var (
		backetname = "test-bucket"
		objectname = "test_csv"
	)
	url, err := controllers.SaveToCloudStorage(backetname, objectname, data)
	assert.NoError(t, err)

	assert.NotEmpty(t, url)

	fmt.Println(url)
}

func TestSepRows(t *testing.T) {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, TESTPROJECTID)
	assert.NoError(t, err)
	defer client.Close()

	docs, err := client.Collection("status_inout").Documents(ctx).GetAll()
	assert.NoError(t, err)

	var rows []map[string]interface{}
	for _, doc := range docs {
		var m map[string]interface{}
		assert.NoError(t, doc.DataTo(&m))
		rows = append(rows, m)
	}

	t.Log(len(rows))

	now := time.Now().UTC().Add(9 * time.Hour)
	this, prev := controllers.SepMapByMonth(now, rows)

	assert.NotEqual(t, len(this), 0)
	assert.NotEqual(t, len(prev), 1)

}
