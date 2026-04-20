package windows

import (
	"encoding/xml"
	"fmt"
)

// SyncML message types for OMA-DM protocol

// SyncML is the root element of an OMA-DM message
type SyncML struct {
	XMLName  xml.Name  `xml:"SYNCML:SYNCML1.2 SyncML"`
	SyncHdr  SyncHdr   `xml:"SyncHdr"`
	SyncBody SyncBody  `xml:"SyncBody"`
}

// SyncHdr contains session and message metadata
type SyncHdr struct {
	VerDTD    string    `xml:"VerDTD"`
	VerProto  string    `xml:"VerProto"`
	SessionID string    `xml:"SessionID"`
	MsgID     string    `xml:"MsgID"`
	Target    *LocURI   `xml:"Target"`
	Source    *LocURI   `xml:"Source"`
}

// LocURI identifies a source or target
type LocURI struct {
	LocURI string `xml:"LocURI"`
}

// SyncBody contains the commands
type SyncBody struct {
	Alert   []SyncMLAlert   `xml:"Alert,omitempty"`
	Status  []SyncMLStatus  `xml:"Status,omitempty"`
	Get     []SyncMLGet     `xml:"Get,omitempty"`
	Results []SyncMLResults `xml:"Results,omitempty"`
	Replace []SyncMLReplace `xml:"Replace,omitempty"`
	Exec    []SyncMLExec    `xml:"Exec,omitempty"`
	Final   *string         `xml:"Final"`
}

// SyncMLAlert represents an Alert command (session init, etc.)
type SyncMLAlert struct {
	CmdID string        `xml:"CmdID"`
	Data  string        `xml:"Data"`
	Item  []SyncMLItem  `xml:"Item,omitempty"`
}

// SyncMLStatus represents a Status response to a command
type SyncMLStatus struct {
	CmdID   string `xml:"CmdID"`
	MsgRef  string `xml:"MsgRef"`
	CmdRef  string `xml:"CmdRef"`
	Cmd     string `xml:"Cmd"`
	Data    string `xml:"Data"` // Status code: 200=OK, 212=auth accepted, etc.
	TargetRef string `xml:"TargetRef,omitempty"`
	SourceRef string `xml:"SourceRef,omitempty"`
}

// SyncMLGet represents a Get command (query a node)
type SyncMLGet struct {
	CmdID string        `xml:"CmdID"`
	Item  []SyncMLItem  `xml:"Item"`
}

// SyncMLResults contains results from a Get command
type SyncMLResults struct {
	CmdID  string        `xml:"CmdID"`
	MsgRef string        `xml:"MsgRef"`
	CmdRef string        `xml:"CmdRef"`
	Item   []SyncMLItem  `xml:"Item"`
}

// SyncMLReplace represents a Replace command (set a value)
type SyncMLReplace struct {
	CmdID string        `xml:"CmdID"`
	Item  []SyncMLItem  `xml:"Item"`
}

// SyncMLExec represents an Exec command (execute an action)
type SyncMLExec struct {
	CmdID string        `xml:"CmdID"`
	Item  []SyncMLItem  `xml:"Item"`
}

// SyncMLItem represents a data item
type SyncMLItem struct {
	Target *LocURI      `xml:"Target,omitempty"`
	Source *LocURI      `xml:"Source,omitempty"`
	Data   string       `xml:"Data,omitempty"`
	Meta   *SyncMLMeta  `xml:"Meta,omitempty"`
}

// SyncMLMeta contains metadata about an item
type SyncMLMeta struct {
	Format *MetaFormat `xml:"Format,omitempty"`
	Type   *MetaType   `xml:"Type,omitempty"`
}

// MetaFormat specifies the data format
type MetaFormat struct {
	XMLNS string `xml:"xmlns,attr,omitempty"`
	Value string `xml:",chardata"`
}

// MetaType specifies the MIME type
type MetaType struct {
	XMLNS string `xml:"xmlns,attr,omitempty"`
	Value string `xml:",chardata"`
}

// OMA-DM status codes
const (
	StatusOK                = "200"
	StatusAuthAccepted      = "212"
	StatusOperationComplete = "200"
	StatusAcceptedForProc   = "202"
	StatusNotFound          = "404"
	StatusCommandFailed     = "500"
)

// OMA-DM alert codes
const (
	AlertClientInitiated = "1201" // Client-initiated session
	AlertServerInitiated = "1200" // Server-initiated session
)

// ParseSyncML parses a SyncML XML message
func ParseSyncML(data []byte) (*SyncML, error) {
	var msg SyncML
	if err := xml.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse SyncML: %w", err)
	}
	return &msg, nil
}

// GenerateSyncML serializes a SyncML message to XML
func GenerateSyncML(msg *SyncML) ([]byte, error) {
	data, err := xml.MarshalIndent(msg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to generate SyncML: %w", err)
	}
	return append([]byte(xml.Header), data...), nil
}

// NewSyncMLResponse creates a server response SyncML message
func NewSyncMLResponse(sessionID, msgID, serverURI, deviceURI string) *SyncML {
	final := ""
	return &SyncML{
		SyncHdr: SyncHdr{
			VerDTD:    "1.2",
			VerProto:  "DM/1.2",
			SessionID: sessionID,
			MsgID:     msgID,
			Target:    &LocURI{LocURI: deviceURI},
			Source:    &LocURI{LocURI: serverURI},
		},
		SyncBody: SyncBody{
			Final: &final,
		},
	}
}

// AddStatus adds a status element to the response body
func (msg *SyncML) AddStatus(cmdID, msgRef, cmdRef, cmd, statusCode string) {
	msg.SyncBody.Status = append(msg.SyncBody.Status, SyncMLStatus{
		CmdID:  cmdID,
		MsgRef: msgRef,
		CmdRef: cmdRef,
		Cmd:    cmd,
		Data:   statusCode,
	})
}

// AddGet adds a Get command to the message body
func (msg *SyncML) AddGet(cmdID string, uris ...string) {
	items := make([]SyncMLItem, len(uris))
	for i, uri := range uris {
		items[i] = SyncMLItem{Target: &LocURI{LocURI: uri}}
	}
	msg.SyncBody.Get = append(msg.SyncBody.Get, SyncMLGet{
		CmdID: cmdID,
		Item:  items,
	})
}

// AddExec adds an Exec command to the message body
func (msg *SyncML) AddExec(cmdID, targetURI string) {
	msg.SyncBody.Exec = append(msg.SyncBody.Exec, SyncMLExec{
		CmdID: cmdID,
		Item: []SyncMLItem{{
			Target: &LocURI{LocURI: targetURI},
		}},
	})
}

// AddReplace adds a Replace command to set a CSP node value.
func (msg *SyncML) AddReplace(cmdID, targetURI, value, format string) {
	item := SyncMLItem{
		Target: &LocURI{LocURI: targetURI},
		Data:   value,
	}
	if format != "" {
		item.Meta = &SyncMLMeta{
			Format: &MetaFormat{XMLNS: "syncml:metinf", Value: format},
		}
	}
	msg.SyncBody.Replace = append(msg.SyncBody.Replace, SyncMLReplace{
		CmdID: cmdID,
		Item:  []SyncMLItem{item},
	})
}

// GetDeviceID extracts the device identifier from the SyncML Source header
func (msg *SyncML) GetDeviceID() string {
	if msg.SyncHdr.Source != nil {
		return msg.SyncHdr.Source.LocURI
	}
	return ""
}
