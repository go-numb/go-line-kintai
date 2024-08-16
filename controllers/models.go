package controllers

import (
	"strings"
)

type TypeStatus int

type TypeCommand int

const (
	_                  TypeStatus = iota
	TypeStatusActive              // 出勤
	TypeStatusInactive            // 退勤
	TypeStatusBreak               // 休憩
	TypeStatusOut                 // 外回り/会議/打ち合わせ
	TypeStatusFocus               // 重要/業務集中/運転中
	TypeStatusDeskwork            // 事務作業
)

func (i TypeStatus) String() string {
	switch i {
	case TypeStatusActive:
		return "出勤"
	case TypeStatusInactive:
		return "退勤"
	case TypeStatusBreak:
		return "休憩"
	case TypeStatusOut:
		return "外回り/会議/打ち合わせ"
	case TypeStatusDeskwork:
		return "事務作業"
	case TypeStatusFocus:
		return "重要/業務集中/運転中"
	}

	return "未定義"
}

func IsStatus(s string) (TypeStatus, bool) {
	s = strings.TrimSpace(s)
	switch s {
	case "出勤":
		return TypeStatusActive, true
	case "退勤":
		return TypeStatusInactive, true
	case "休憩":
		return TypeStatusBreak, true
	case "外回り/会議/打ち合わせ":
		return TypeStatusOut, true
	case "事務作業":
		return TypeStatusDeskwork, true
	case "重要/業務集中/運転中":
		return TypeStatusFocus, true
	}

	return 0, false

}

const (
	_                 TypeCommand = iota
	TypeCommandHelp               // ヘルプを表示
	TypeCommandAll                // 全員分のステータスを返す
	TypeCommandMe                 // 自分のステータスを返す
	TypeCommandAgg                // 集計 gcsのファイルリンクを返す
	TypeCommandModify             // 残業時間を追加する
)

func IsCommand(s string) (TypeCommand, bool) {
	command := strings.ToLower(strings.TrimSpace(s))
	switch command {
	case "help":
		return TypeCommandHelp, true
	case "all":
		return TypeCommandAll, true
	case "me":
		return TypeCommandMe, true
	case "agg!":
		return TypeCommandAgg, true
	default:
		// modify! で始まる場合
		if strings.HasPrefix(s, "modify!") {
			return TypeCommandModify, true
		}

	}

	return 0, false
}

func (c TypeCommand) String() string {
	switch c {
	case TypeCommandHelp:
		return "help"
	case TypeCommandAll:
		return "all"
	case TypeCommandMe:
		return "me"
	case TypeCommandAgg:
		return "agg!"
	case TypeCommandModify:
		return "modify!"
	}

	return "未定義"
}

func (c TypeCommand) Help() string {
	return `使い方: 以下のコマンドを入力してください
- help: 使い方を表示
- all: 全員分のステータスを表示
- me: 自分のステータスを表示
- agg!: 労働時間集計を表示
- modify!: 労働時間記録に補足・修正の追記を行う`
}
