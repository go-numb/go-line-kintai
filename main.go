package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/go-numb/go-line-kintai/controllers"
	"github.com/rs/xid"

	"github.com/labstack/echo/v4"
	"github.com/line/line-bot-sdk-go/v8/linebot/messaging_api"
	"github.com/line/line-bot-sdk-go/v8/linebot/webhook"

	"cloud.google.com/go/firestore"
)

const (
	TZOFFSET            = 9 * time.Hour
	DB_COL_STATUS       = "status"
	DB_COL_STATUS_INOUT = "status_inout"
)

var (
	PORT          = ""
	ACCESSTOKEN   = ""
	CHANNELSECRET = ""
)

func init() {
	if runtime.GOOS == "windows" {
		PORT = "8081"
	} else if runtime.GOOS == "linux" {
		PORT = os.Getenv("PORT")
		ACCESSTOKEN = os.Getenv("ACCESSTOKEN")
		CHANNELSECRET = os.Getenv("CHANNELSECRET")
	}
}

func main() {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	e.POST("/callback", func(c echo.Context) error {
		client := &http.Client{}
		bot, err := messaging_api.NewMessagingApiAPI(
			ACCESSTOKEN,
			messaging_api.WithHTTPClient(client),
		)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}

		whook, err := webhook.ParseRequest(CHANNELSECRET, c.Request())
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}

		for _, event := range whook.Events {
			switch ev := event.(type) {
			case webhook.MessageEvent:
				switch message := ev.Message.(type) {
				case webhook.TextMessageContent:
					status, isStatus := controllers.IsStatus(message.Text)
					if !isStatus {
						if err := replay(bot, ev, message.Text); err != nil {
							log.Println(err)
							return c.JSON(http.StatusBadRequest, err)
						}
						break
					}

					if err := updateStatus(bot, ev, status); err != nil {
						if _, e := bot.ReplyMessage(&messaging_api.ReplyMessageRequest{
							ReplyToken: ev.ReplyToken,
							Messages: []messaging_api.MessageInterface{
								&messaging_api.TextMessage{
									Text: fmt.Sprintf("error: %+v", err),
								},
							},
						}); e != nil {
							err = errors.Join(err, e)
						}

						return c.JSON(http.StatusBadRequest, err)
					}

				}

			default:
				if _, err := bot.ReplyMessage(&messaging_api.ReplyMessageRequest{
					Messages: []messaging_api.MessageInterface{
						&messaging_api.TextMessage{
							Text: fmt.Sprintf("Hello, World! default: %+v", ev),
						},
					}}); err != nil {
					return c.JSON(http.StatusBadRequest, err)
				}
			}
		}

		return c.String(http.StatusOK, "Hello, World! callback")
	})

	e.Logger.Fatal(e.Start("0.0.0.0:" + PORT))
}

func updateStatus(bot *messaging_api.MessagingApiAPI, ev webhook.MessageEvent, status controllers.TypeStatus) error {
	var userId string
	switch s := ev.Source.(type) {
	case webhook.UserSource:
		userId = s.UserId
	}

	// 9時間の時差を補正
	ev.Timestamp = controllers.ToJSTime(ev.Timestamp)
	changeAt := time.Unix(ev.Timestamp/1000, 0).Format("15:04:05")

	dname, recent, recentAt, err := setUserStatus(userId, status.String())
	if err != nil {
		log.Println(err)
		return err
	}

	if _, err := bot.ReplyMessage(&messaging_api.ReplyMessageRequest{
		ReplyToken: ev.ReplyToken,
		Messages: []messaging_api.MessageInterface{
			&messaging_api.TextMessage{
				Text: fmt.Sprintf("[ 更新 ]\n%s: %s\n%s->\n%s\n%s\n%s", dname, controllers.MaskString(userId), recentAt.Format("15:04:05"), recent, changeAt, status.String()),
			},
		}}); err != nil {
		return err
	}

	return nil
}

// setUserStatus 引数を保存し、前回のステータスを返す
func setUserStatus(userId, status string) (string, string, time.Time, error) {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, os.Getenv("PROJECTID"))
	if err != nil {
		return "", "", time.Time{}, err
	}
	defer client.Close()

	dname, err := getUser(userId)
	if err != nil {
		return "", "", time.Time{}, err
	}

	doc, _ := client.Collection(DB_COL_STATUS).Doc(userId).Get(ctx)

	var m map[string]interface{}
	doc.DataTo(&m)

	utc := time.Now().UTC()

	if _, err := client.Collection(DB_COL_STATUS).Doc(userId).Set(ctx, map[string]interface{}{
		"user_id":      userId,
		"display_name": dname,
		"status":       status,
		"start_at":     utc.UnixMilli(),
	}); err != nil {
		return "", "", time.Time{}, err
	}

	if status == controllers.TypeStatusActive.String() ||
		status == controllers.TypeStatusInactive.String() {
		// 労働時間集計用: 出勤、退勤のみ記録
		x := xid.New().String() // 一意のID sort可能
		y, month, d := utc.Add(TZOFFSET).Date()
		weekday := controllers.WeekdayToString(utc.Add(TZOFFSET).Weekday())
		h, minites, _ := utc.Add(TZOFFSET).Clock()

		if _, err := client.Collection(DB_COL_STATUS_INOUT).Doc(x).Set(ctx, map[string]interface{}{
			"year":    y,
			"month":   int(month),
			"day":     d,
			"weekday": weekday,
			"hour":    h,
			"minites": minites,

			"user_id":      userId,
			"display_name": dname,
			"status":       status,
			"note":         "",
			"timestamp":    utc.UnixMilli(),
		}); err != nil {
			return "", "", time.Time{}, err
		}

	}

	var startAt time.Time
	if v, isThere := m["start_at"].(int64); isThere {
		v = v + 9*60*60*1000
		startAt = time.Unix(v/1000, 0)
	}

	return dname, m["status"].(string), startAt, nil
}

// setModify 修正コマンド用
func setModify(userId, msg string) (string, error) {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, os.Getenv("PROJECTID"))
	if err != nil {
		return "", err
	}
	defer client.Close()

	dname, err := getUser(userId)
	if err != nil {
		return "", err
	}

	// 労働時間集計用: 出勤、退勤のみ記録
	// 備考欄への記載
	x := xid.New().String() // 一意のID sort可能
	utc := time.Now().UTC()
	y, month, d := utc.Add(TZOFFSET).Date()
	weekday := controllers.WeekdayToString(utc.Add(TZOFFSET).Weekday())
	h, minites, _ := utc.Add(TZOFFSET).Clock()

	if _, err := client.Collection(DB_COL_STATUS_INOUT).Doc(x).Set(ctx, map[string]interface{}{
		"year":    y,
		"month":   int(month),
		"day":     d,
		"weekday": weekday,
		"hour":    h,
		"minites": minites,

		"user_id":      userId,
		"display_name": dname,
		"status":       "修正/備考",
		"note":         msg,
		"timestamp":    utc.UnixMilli(),
	}); err != nil {
		return "", err
	}

	return x, nil
}

// getAllStatus 全員のステータスを取得
func getAllStatus() (string, error) {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, os.Getenv("PROJECTID"))
	if err != nil {
		return "", err
	}
	defer client.Close()

	docs, err := client.Collection(DB_COL_STATUS).Documents(ctx).GetAll()
	if err != nil {
		return "", err
	}

	var maps []map[string]interface{}
	for _, doc := range docs {
		var m map[string]interface{}
		doc.DataTo(&m)
		maps = append(maps, m)
	}

	// sort
	sort.Slice(maps, func(i, j int) bool {
		return maps[i]["display_name"].(string) < maps[j]["display_name"].(string)
	})

	// 整形
	var result string
	for _, m := range maps {
		result += fmt.Sprintf("- %s: %s\n", m["display_name"].(string), m["status"].(string))
	}

	return result, nil
}

// getStatus 自身のステータスを取得
func getStatus(userId string) (string, error) {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, os.Getenv("PROJECTID"))
	if err != nil {
		return "", err
	}
	defer client.Close()

	displayName, err := getUser(userId)
	if err != nil {
		return "", err
	}

	doc, err := client.Collection(DB_COL_STATUS).Doc(userId).Get(ctx)
	if err != nil {
		return "", err
	}

	var m map[string]interface{}
	doc.DataTo(&m)

	status, isThere := m["status"].(string)
	if !isThere {
		return "", errors.New("status not found")
	}

	return fmt.Sprintf("- %s: %s\n", displayName, status), nil
}

// getAllHistory 履歴を取得
func getAllHistory() ([]map[string]interface{}, error) {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, os.Getenv("PROJECTID"))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	docs, err := client.Collection(DB_COL_STATUS_INOUT).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	var maps []map[string]interface{}
	for _, doc := range docs {
		var m map[string]interface{}
		doc.DataTo(&m)
		maps = append(maps, m)
	}

	return maps, nil
}

func getUser(userId string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.line.me/v2/bot/profile/"+userId, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ACCESSTOKEN))

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var m map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		return "", err
	}

	return m["displayName"].(string), nil
}

func replay(bot *messaging_api.MessagingApiAPI, ev webhook.MessageEvent, msg string) error {
	var userId string
	switch s := ev.Source.(type) {
	case webhook.UserSource:
		userId = s.UserId
	}

	var toMsg string
	command, _ := controllers.IsCommand(msg)
	switch command {
	case controllers.TypeCommandHelp:
		toMsg = command.Help()

	case controllers.TypeCommandAll:
		s, err := getAllStatus()
		if err != nil {
			return err
		}
		toMsg = fmt.Sprintf("全員のステータス\n%s", s)

	case controllers.TypeCommandMe:
		s, err := getStatus(userId)
		if err != nil {
			return err
		}
		toMsg = fmt.Sprintf("自身のステータス\n%s", s)

	case controllers.TypeCommandAgg:
		maps, err := getAllHistory()
		if err != nil {
			return err
		}

		// 当月及び前月のデータを分類
		now := time.Now().UTC().Add(TZOFFSET)
		thisMonthRows, prevMonthRows := controllers.SepMapByMonth(now, maps)

		toMsg = "[ 出退勤ファイルリンク ]\n"

		if len(thisMonthRows) != 0 {
			thisMonthByte, err := controllers.ToCsvByte(thisMonthRows)
			if err != nil {
				return err
			}
			thisMonthMedialink, err := controllers.SaveToCloudStorage(os.Getenv("BUCKETNAME"), fmt.Sprintf("history_thismonth_%s.csv", now.Format("200601021504")), thisMonthByte)
			if err != nil {
				return err
			}

			toMsg += fmt.Sprintf("当月: %s\n", thisMonthMedialink)
		}

		if len(prevMonthRows) != 0 {
			prevMonthByte, err := controllers.ToCsvByte(prevMonthRows)
			if err != nil {
				return err
			}
			prevMonthMedialink, err := controllers.SaveToCloudStorage(os.Getenv("BUCKETNAME"), fmt.Sprintf("history_prevmonth_%s.csv", now.Format("200601021504")), prevMonthByte)
			if err != nil {
				return err
			}

			toMsg += fmt.Sprintf("前月: %s\n", prevMonthMedialink)
		}

		toMsg += fmt.Sprintf("出力日時: %s", now.Format("2006/01/02 15:04:05"))

	case controllers.TypeCommandModify:
		// 修正コマンド
		removedStr := strings.Replace(msg, controllers.TypeCommandModify.String(), "", -1)
		sid, err := setModify(userId, removedStr)
		if err != nil {
			return err
		}
		toMsg = fmt.Sprintf("[ 修正完了 ]\nid: %s\nmodified by %s", sid, controllers.MaskString(userId))

	default:
		// 未定義のワードが入力された場合
		// Bot不介入、会話として無視する
		// toMsg = fmt.Sprintf("未定義: %s", msg)
		return nil
	}

	if _, err := bot.ReplyMessage(&messaging_api.ReplyMessageRequest{
		ReplyToken: ev.ReplyToken,
		Messages: []messaging_api.MessageInterface{
			&messaging_api.TextMessage{
				Text: toMsg,
			},
		}}); err != nil {
		return err
	}

	return nil
}
