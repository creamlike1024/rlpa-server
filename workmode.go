package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type RLPAWorkMode interface {
	Start(client *RLPAClient)
	OnProcessFinished(client *RLPAClient, data *Payload)
	Finished() bool
}

type ShellWorkMode struct {
}

func (m *ShellWorkMode) Start(c *RLPAClient) {
	// 添加到 Client 列表并发送 ID 和密码
	APIClients = append(APIClients, c)
	passwd := c.GenCredential()
	err := c.MessageBox(fmt.Sprintf("ManageID: %s\nPassword: %s", c.ID, passwd))
	if err != nil {
		c.ErrLog(err.Error())
		c.Close(ResultError)
		return
	}
}

func (m *ShellWorkMode) OnProcessFinished(c *RLPAClient, data *Payload) {
	// TODO 完成，发送结果 json
	resp, err := json.Marshal(data)
	if err != nil {
		c.ResponseChan <- []byte(err.Error())
		return
	}
	if c.ResponseWaiting {
		c.ResponseChan <- resp
		return
	}

}

func (m *ShellWorkMode) Finished() bool {
	return false
}

type ProcessNotificationWorkMode struct {
	State         int
	FailedCount   int
	TotalCount    int
	Notifications []*Notification
}

func (m *ProcessNotificationWorkMode) Start(c *RLPAClient) {
	m.State = 0
	err := c.processOpenLpac("notification", "list")
	if err != nil {
		c.Close(ResultError)
		return
	}
}

func (m *ProcessNotificationWorkMode) OnProcessFinished(c *RLPAClient, data *Payload) {
	if data == nil {
		c.Close(ResultError)
		return
	}
	switch m.State {
	case 0:
		if data.Code != 0 {
			c.Close(ResultError)
			return
		}
		err := json.Unmarshal(data.Data, &m.Notifications)
		if err != nil {
			c.Close(ResultError)
			return
		}
		m.State = 1
		m.TotalCount = len(m.Notifications)
		m.processOneNotification(c)
		break
	case 1:
		if data.Code == 0 {
			c.InfoLog("Process success")
		} else {
			c.InfoLog("Process failed")
			m.FailedCount++
		}
		m.processOneNotification(c)
		break
	}
}

func (m *ProcessNotificationWorkMode) Finished() bool {
	return m.State == 2
}

func (m *ProcessNotificationWorkMode) processOneNotification(c *RLPAClient) {
	// 如果没有通知，输出结果，并切换状态
	if len(m.Notifications) == 0 {

		if m.FailedCount != 0 {
			c.InfoLog(fmt.Sprint(m.FailedCount, "notification failed to process"))
		} else {
			c.InfoLog("All notification process successfully")
		}
		err := c.MessageBox(fmt.Sprint("All notification processing finished\n", m.TotalCount-m.FailedCount, " succeed\n", m.FailedCount, " failed"))
		if err != nil {
			c.Close(ResultError)
		} else {
			c.Close(ResultFinished)
		}
		m.State = 2
		return
	}
	notification := m.Notifications[0]
	m.Notifications = m.Notifications[1:]
	c.InfoLog(fmt.Sprint("Processing ", notification.SeqNumber, " Operation: ", notification.ProfileManagementOperation))
	switch notification.ProfileManagementOperation {
	case "install":
		fallthrough
	case "enable":
		fallthrough
	case "disable":
		err := c.processOpenLpac("notification", "process", strconv.Itoa(notification.SeqNumber), "-r")
		if err != nil {
			c.Close(ResultError)
		}
		break
	case "delete":
		err := c.processOpenLpac("notification", "process", strconv.Itoa(notification.SeqNumber))
		if err != nil {
			c.Close(ResultError)
		}
		break
	default:
		c.Close(ResultError)
	}
}

type DownloadWorkMode struct {
	State int
}

func (m *DownloadWorkMode) Start(c *RLPAClient) {
	m.State = 0
	// 替换所有的 \x02 (STX) 字符为 $
	data := strings.Replace(string(c.Packet.Value), string([]byte{0x02}), "$", -1)
	// 替换所有的 \x11 (DC1) 字符为 _
	data = strings.Replace(data, string([]byte{0x11}), "_", -1)
	data = strings.TrimSpace(data)
	pullInfo, confirmCodeNeeded, err := DecodeLpaActivationCode(CompleteActivationCode(data))
	if err != nil {
		_ = c.MessageBox(err.Error())
		c.Close(ResultError)
		return
	}
	if confirmCodeNeeded {
		_ = c.MessageBox("Confirm Code is not supported yet")
		c.Close(ResultFinished)
		return
	}
	args := []string{"profile", "download"}
	if pullInfo.SMDP != "" {
		args = append(args, "-s", pullInfo.SMDP)
	}
	if pullInfo.MatchID != "" {
		args = append(args, "-m", pullInfo.MatchID)
	}
	err = c.processOpenLpac(args...)
	if err != nil {
		c.ErrLog(err.Error())
		c.Close(ResultError)
	}
}

func (m *DownloadWorkMode) OnProcessFinished(c *RLPAClient, data *Payload) {
	if data == nil {
		c.Close(ResultError)
		return
	}
	if data.Code == 0 {
		c.InfoLog("Download success")
		_ = c.MessageBox("Download success")
		c.Close(ResultFinished)
	} else {
		c.ErrLog("download error: " + string(data.Data))
		// TODO 解析错误信息
		// 也许把 EasyLPAC 代码搬过来
		errorMsg := fmt.Sprint("Data: ", data.Data)
		_ = c.MessageBox(errorMsg)
		c.Close(ResultError)
	}
	m.State = 1
}

func (m *DownloadWorkMode) Finished() bool {
	return m.State == 1
}
