package controllers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"cloud.google.com/go/storage"
)

// MaskString is a function that masks the string.
func MaskString(s string) string {
	var l = len(s)
	if l < 5 {
		return s
	}

	return s[l-4:]
}

// time.Weekday to [月, 火, 水, 木, 金, 土, 日]
func WeekdayToString(w time.Weekday) string {
	switch w {
	case time.Monday:
		return "月"
	case time.Tuesday:
		return "火"
	case time.Wednesday:
		return "水"
	case time.Thursday:
		return "木"
	case time.Friday:
		return "金"
	case time.Saturday:
		return "土"
	case time.Sunday:
		return "日"
	}
	return ""
}

func ToCsvByte(rows []map[string]interface{}) ([]byte, error) {
	var keys []string
	for k := range rows[0] {
		keys = append(keys, k)
	}

	var data [][]string
	data = append(data, keys)
	for _, row := range rows {
		var d []string
		for _, k := range keys {
			d = append(d, fmt.Sprintf("%v", row[k]))
		}
		data = append(data, d)
	}

	var s string
	for _, d := range data {
		s += strings.Join(d, ",") + "\n"
	}

	return []byte(s), nil
}

// save to cloud storage
func SaveToCloudStorage(bucketname, objectname string, data []byte) (string, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Println(err)
		return "", err
	}
	defer client.Close()

	object := client.Bucket(bucketname).Object(objectname)

	w := object.NewWriter(ctx)
	defer w.Close()

	w.ContentType = "text/csv"
	if _, err := w.Write(data); err != nil {
		log.Println(err)
		return "", err
	}

	time.Sleep(time.Second)
	for i := 0; i < 10; i++ {
		if err := w.Close(); err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	// create file link
	attrs, err := object.Attrs(ctx)
	if err != nil {
		log.Println(err)
		return "", err
	}

	return attrs.MediaLink, nil
}

func ToJSTime(i int64) int64 {
	return i + 9*60*60*1000
}

// SepMapByMonth 当月と先月のデータを分ける
func SepMapByMonth(now time.Time, rows []map[string]any) ([]map[string]any, []map[string]any) {
	var (
		thisMonth                    = now.Month()
		prevMonth                    = now.Add(-15 * 24 * time.Hour).Month()
		thisMonthRows, prevMonthRows = make([]map[string]any, 0), make([]map[string]any, 0)
	)
	for _, row := range rows {
		month, isThere := row["month"].(int64)
		if !isThere {
			continue
		}

		if int(thisMonth) == int(month) {
			thisMonthRows = append(thisMonthRows, row)
		} else if int(prevMonth) == int(month) {
			prevMonthRows = append(prevMonthRows, row)
		}
	}

	return thisMonthRows, prevMonthRows
}
