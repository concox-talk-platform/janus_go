/**
 * Copyright (c) 2019. All rights reserved.
 * Author: tesion
 * Date: April 15th 2019
 */
package janus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

const (
	PostContentType = "application/json;charset=utf-8"
	TransLen = 12
	IntervalSec = 1
)

var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

type Client struct {
	SessionId	uint64
	HandleId	uint64
	Plugin		string
	Url			string
	Mtx			sync.Mutex
	Gw 			sync.WaitGroup
	Interval 	uint64
}

type CreateReq struct {
	Janus string `json:"janus"`
	Trans string `json:"transaction"`
}

func getRandString(n int) string {
	b := make([]rune, n)
	randLen := len(letterRunes)
	for i := range b {
		b[i] = letterRunes[rand.Intn(randLen)]
	}

	return string(b)
}

func NewClient() *Client {
	cli := &Client{Interval:IntervalSec}
	return cli
}

func (cli *Client) GetSessionId() uint64 {
	cli.Mtx.Lock()
	defer cli.Mtx.Unlock()

	return cli.SessionId
}

func (cli *Client) SetSessionId(sess uint64) {
	cli.Mtx.Lock()
	defer cli.Mtx.Unlock()

	cli.SessionId = sess
}

func (cli *Client) GetHandleId() uint64 {
	cli.Mtx.Lock()
	defer cli.Mtx.Unlock()

	return cli.HandleId
}

func (cli *Client) SetHandleId(handle uint64) {
	cli.Mtx.Lock()
	defer cli.Mtx.Unlock()

	cli.HandleId = handle
}

func (cli *Client) SetInterval(interval uint64) {
	cli.Mtx.Lock()
	defer cli.Mtx.Unlock()

	cli.Interval = interval
}

func (cli *Client) GetInterval() uint64 {
	cli.Mtx.Lock()
	defer cli.Mtx.Unlock()

	return cli.Interval
}

func (cli *Client) SetUrl(url string) {
	cli.Url = url
}

func (cli *Client) PostData(req []byte) ([]byte, error) {
	res, err := http.Post(cli.Url, PostContentType, bytes.NewBuffer(req))
	if err != nil {
		return nil, fmt.Errorf("post error: %s", err.Error())
	}

	defer res.Body.Close()

	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read response error: %s", err.Error())
	}

	return content, nil
}

func (cli *Client) Connect(url string) error {
	cli.Url = url
	var req CreateReq
	req.Janus = "create"
	req.Trans = getRandString(TransLen)

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("json encode error: %s", err.Error())
	}

	//res, err := http.Post(cli.Url, PostContentType, bytes.NewBuffer(data))
	//if err != nil {
	//	return fmt.Errorf("post error: %s", err.Error())
	//}
	//
	//defer res.Body.Close()
	//
	//content, err := ioutil.ReadAll(res.Body)
	//if err != nil {
	//	return fmt.Errorf("read error: %s", err.Error())
	//}

	content, err := cli.PostData(data)
	if err != nil {
		return err
	}

	var rsp map[string]interface{}
	if err = json.Unmarshal(content, &rsp); err != nil {
		return fmt.Errorf("json decode error: %s", err.Error())
	}

	stat := rsp["janus"]
	if stat != "success" {
		return fmt.Errorf("janus error, response(%s)", content)
	}

	sess := rsp["data"].(map[string]interface{})
	cli.SetSessionId(uint64(sess["id"].(float64)))

	return nil
}

func (cli *Client) Run() {
	sessId := cli.GetSessionId()
	if 0 == sessId {
		log.Fatalf("invalid session id(%d)", sessId)
	}

	cli.Gw.Add(1)
	go cli.DoKeepAlive()
}

/**
 * if client attched some plugin, must detach first, then close the session
 */
func (cli *Client) Close() error {
	sessId := cli.GetSessionId()
	if sessId == 0 {
		return fmt.Errorf("invalid session: %d", sessId)
	}

	handleId := cli.GetHandleId()
	if handleId != 0 {
		// detach plugin
		if err := cli.Detach(); err != nil {
			return err
		}
	}

	req := make(map[string]interface{})
	req["janus"] = "destroy"
	req["transaction"] = getRandString(TransLen)
	req["session_id"] = sessId

	if 0 != cli.HandleId {
		req["handle_id"] = cli.HandleId
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("json encode error: %s", err.Error())
	}

	content, err := cli.PostData(data)
	if err != nil {
		return err
	}

	var rsp map[string]interface{}
	if err = json.Unmarshal(content, &rsp); err != nil {
		return fmt.Errorf("json decode error: %s", err.Error())
	}

	stat := rsp["janus"]
	if stat != "success" {
		return fmt.Errorf("janus error, response(%s)", content)
	}

	cli.SetSessionId(0)
	cli.SetHandleId(0)

	return nil
}

func (cli *Client) Attach(plugin string) error {
	handleId := cli.GetHandleId()
	if 0 != handleId {
		return fmt.Errorf("client have been attached handle(%d)", handleId)
	}

	req := make(map[string]interface{})
	req["janus"] = "attach"
	req["transaction"] = getRandString(TransLen)
	req["session_id"] = cli.GetSessionId()
	req["plugin"] = plugin

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("json encode error: ", err.Error())
	}

	content, err := cli.PostData(data)
	if err != nil {
		return nil
	}

	var rsp map[string]interface{}
	if err = json.Unmarshal(content, &rsp); err != nil {
		return fmt.Errorf("json decode error: %s", err.Error())
	}

	stat := rsp["janus"]
	if stat != "success" {
		return fmt.Errorf("janus error, response(%s)", content)
	}

	handle := rsp["data"].(map[string]interface{})
	cli.SetHandleId(uint64(handle["id"].(float64)))
	cli.Plugin = plugin
	return nil
}

func (cli *Client) Detach() error {
	handleId := cli.GetHandleId()
	if 0 == handleId {
		return fmt.Errorf("invalid handle id(%d)", handleId)
	}

	req := make(map[string]interface{})
	req["janus"] = "detach"
	req["transaction"] = getRandString(TransLen)
	req["session_id"] = cli.GetSessionId()
	req["handle_id"] = handleId

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("json encode error: ", err.Error())
	}

	content, err := cli.PostData(data)
	if err != nil {
		return err
	}

	var rsp map[string]interface{}
	if err = json.Unmarshal(content, &rsp); err != nil {
		return fmt.Errorf("json decode error: %s", err.Error())
	}

	stat := rsp["janus"]
	if stat != "success" {
		return fmt.Errorf("janus detach plugin(%s) error, response(%s)", cli.Plugin, content)
	}

	cli.Plugin = ""
	cli.SetHandleId(0)
	return nil
}

func (cli *Client) KeepAlive() error {
	sessId := cli.GetSessionId()
	if 0 == sessId {
		return fmt.Errorf("invalid session(%d)", sessId)
	}

	req := make(map[string]interface{})
	req["janus"] = "keepalive"
	req["transaction"] = getRandString(TransLen)
	req["session_id"] = sessId

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("json encode error: ", err.Error())
	}

	content, err := cli.PostData(data)
	if err != nil {
		return err
	}

	var rsp map[string]interface{}
	if err = json.Unmarshal(content, &rsp); err != nil {
		return fmt.Errorf("json decode error: %s", err.Error())
	}

	stat := rsp["janus"]
	if stat != "ack" {
		return fmt.Errorf("janus error, response(%s)", content)
	}

	return nil
}

func (cli *Client) DoKeepAlive() {
	for {
		if 0 == cli.GetSessionId() {
			// session is closed
			cli.Gw.Done()
			return
		}

		if err := cli.KeepAlive(); err != nil {
			log.Println(err)
		}

		time.Sleep(time.Duration(cli.GetInterval()) * time.Second)
	}
}

func (cli *Client) Stop() {
	cli.Gw.Wait()
}
