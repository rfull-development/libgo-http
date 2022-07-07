// Copyright (c) 2022 RFull Development
// This source code is managed under the MIT license. See LICENSE in the project root.
package conv

import (
	"bufio"
	"encoding/json"
	"errors"
	"log"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Formatは入出力フォーマットの列挙型です。
type Format int

const (
	JsonFormat Format = iota // JSON形式のフォーマットです。
)

// Format型を文字列に変換します。
func (f Format) String() string {
	s := ""
	switch f {
	case JsonFormat:
		s = "json"
	}
	return s
}

// HttpHeaderConverterは動作に関する情報を保持します。
type HttpHeaderConverter struct {
	rawHeader    *os.File // 生HTTPヘッダです。
	outputFormat Format   // 出力フォーマットです。
	numWorker    int      // ワーカー数です。
}

// SetRawHeaderは生HTTPヘッダ入力元を設定します。
func (conv *HttpHeaderConverter) SetRawHeader(rawHeader *os.File) {
	conv.rawHeader = rawHeader
}

// SetOutputFormatは標準出力フォーマットを指定します。
func (conv *HttpHeaderConverter) SetOutputFormat(outputFormat Format) {
	conv.outputFormat = outputFormat
}

// SetNumWorkerはワーカー数を指定します。
func (conv *HttpHeaderConverter) SetNumWorker(numWorker int) {
	conv.numWorker = numWorker
}

// NewHttpHeaderConverterはHttpHeaderConverterのインスタンスを生成します。
func NewHttpHeaderConverter() *HttpHeaderConverter {
	conv := &HttpHeaderConverter{}
	conv.SetRawHeader(os.Stdin)
	conv.SetOutputFormat(JsonFormat)
	maxCpu := runtime.NumCPU()
	conv.SetNumWorker(maxCpu)
	return conv
}

var keyReplacer = strings.NewReplacer("-", " ", "/", " ")

// createJsonKeyはJSONキー名を返却します。
// JSONキー名はHTTPヘッダのキーより不要な文字を除去し、キャメルケースに変換した結果です。
func (conv *HttpHeaderConverter) createJsonKey(key string) (string, error) {
	t := keyReplacer.Replace(key)
	c := cases.Title(language.Und)
	t = c.String(t)
	tl := strings.Split(t, " ")
	tl[0] = strings.ToLower(tl[0])
	t = strings.Join(tl, "")
	return t, nil
}

var pairPattern *regexp.Regexp = regexp.MustCompile(`^(?P<key>.+?):\s+(?P<value>.+)$`)
var pairKeyIndex = pairPattern.SubexpIndex("key")
var pairValueIndex = pairPattern.SubexpIndex("value")

// parsePairはキーと値のペアを送信します。
// 先頭に半角スペースを含まない、かつ、キーと値がペアで定義されているテキストを解釈します。
// キーは出力フォーマットに合わせて変換します。
func (conv *HttpHeaderConverter) parsePair(line string, stream chan<- []string) error {
	if line[0:1] == " " {
		return errors.New("not pair string")
	}
	g := pairPattern.FindStringSubmatch(line)
	if len(g) != 3 {
		return errors.New("cannot parse")
	}
	k := g[pairKeyIndex]
	v := g[pairValueIndex]
	var e error
	switch conv.outputFormat {
	case JsonFormat:
		k, e = conv.createJsonKey(k)
		if e != nil {
			return errors.New("cannot convert JSON key")
		}
		break
	}
	kv := []string{k, v}
	stream <- kv
	return nil
}

var statusPattern *regexp.Regexp = regexp.MustCompile(`(?P<code>[0-9]{3})\s(?P<message>.+)$`)
var statusCodeIndex = statusPattern.SubexpIndex("code")
var statusMessageIndex = statusPattern.SubexpIndex("message")

func (conv *HttpHeaderConverter) parseStatus(line string, stream chan<- []string) error {
	g := statusPattern.FindStringSubmatch(line)
	log.Println(g)
	if len(g) != 3 {
		return errors.New("cannot parse")
	}
	var k string
	v := g[statusCodeIndex]
	switch conv.outputFormat {
	case JsonFormat:
		k = "code"
	default:
		k = "Code"
	}
	kv := []string{k, v}
	stream <- kv
	return nil
}

// createSendersは1行単位でチャンネルに文字列配列を送信します。
// 変換可否によって出力するチャンネルを選択します。
func (conv *HttpHeaderConverter) createSenders(reader *bufio.Reader) (<-chan []string, <-chan []string, error) {
	convertedStream := make(chan []string, conv.numWorker)
	notConvertedStream := make(chan []string, conv.numWorker)
	go func() {
		defer close(convertedStream)
		defer close(notConvertedStream)
		f := true
		for {
			l, e := reader.ReadString('\n')
			if e != nil {
				break
			}
			l = strings.TrimRightFunc(l, unicode.IsSpace)
			if len(l) < 4 {
				continue
			}
			if f {
				f = false
				e = conv.parseStatus(l, convertedStream)
				if e == nil {
					continue
				}
			}
			e = conv.parsePair(l, convertedStream)
			if e == nil {
				continue
			}
			notConvertedStream <- []string{l}
		}
	}()
	return convertedStream, notConvertedStream, nil
}

// convertは変換した結果を返却します。
func (conv *HttpHeaderConverter) convert() (map[string]interface{}, []string, time.Duration, error) {
	// 生ヘッダ送信ワーカー生成
	beginTime := time.Now()
	r := bufio.NewReader(conv.rawHeader)
	convetedStream, notConvertedStream, _ := conv.createSenders(r)

	// 処理済み文字列配列マップ化
	wg := &sync.WaitGroup{}
	receiver := func(stream <-chan []string, receiver func(...string)) {
		defer wg.Done()
		for s := range stream {
			receiver(s...)
		}
	}
	converted := make(map[string]interface{})
	wg.Add(1)
	go receiver(convetedStream, func(s ...string) {
		converted[s[0]] = s[1]
	})
	notConveted := make([]string, 0)
	wg.Add(1)
	go receiver(notConvertedStream, func(s ...string) {
		notConveted = append(notConveted, s[0])
	})
	wg.Wait()

	// 実行時間算出
	endTime := time.Now()
	processTime := endTime.Sub(beginTime)
	return converted, notConveted, processTime, nil
}

// Outputは変換した結果を返却します。
func (conv *HttpHeaderConverter) Output() (string, error) {
	// マップ生成
	converted, notConverted, p, e := conv.convert()
	if e != nil {
		return "", e
	}
	log.Println(p)

	// 出力フォーマット処理
	if len(notConverted) > 0 {
		var k string
		switch conv.outputFormat {
		case JsonFormat:
			k = "raw"
		default:
			k = "Raw"
		}
		converted[k] = notConverted
	}
	t, _ := json.Marshal(converted)
	resp := string(t)
	return resp, nil
}
