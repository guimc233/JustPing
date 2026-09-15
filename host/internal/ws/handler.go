package ws

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
	"github.com/guimc233/JustPing/shared/protocol"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 64,
	WriteBufferSize: 1024 * 64,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// HandleWebSocket handles incoming agent WebSocket connections
func (h *Hub) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] Upgrade failed: %v\n", err)
		return
	}

	ac := &AgentConn{
		Conn: conn,
		Send: make(chan []byte, 256),
		Hub:  h,
	}

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var initialEnv protocol.Envelope
	err = conn.ReadJSON(&initialEnv)
	if err != nil || initialEnv.Type != protocol.TypeRegisterRequest {
		log.Printf("[WS] Handshake failed or invalid initial packet\n")
		_ = writeDirectEnvelope(conn, protocol.TypeError, "First message must be register_request")
		_ = conn.Close()
		return
	}

	rawPayload, _ := json.Marshal(initialEnv.Payload)
	var regReq protocol.RegisterRequest
	if err := json.Unmarshal(rawPayload, &regReq); err != nil || regReq.Token == "" {
		_ = writeDirectEnvelope(conn, protocol.TypeError, "Invalid register payload")
		_ = conn.Close()
		return
	}

	var agent model.Agent
	if err := db.DB.Where("token = ?", regReq.Token).First(&agent).Error; err != nil {
		h := sha256.Sum256([]byte(regReq.Token))
		tokenHash := hex.EncodeToString(h[:8])
		log.Printf("[WS] Authentication rejected for token hash %s: not found\n", tokenHash)
		_ = writeDirectEnvelope(conn, protocol.TypeError, "Invalid or revoked agent token")
		_ = conn.Close()
		return
	}

	remoteIP := c.ClientIP()

	agent.IsOnline = true
	agent.LastSeenAt = time.Now()
	agent.PublicIP = remoteIP
	agent.OS = regReq.OS
	agent.Arch = regReq.Arch
	agent.Version = regReq.Version
	if agent.Name == "" || agent.Name == "New Agent" {
		if regReq.Hostname != "" {
			agent.Name = regReq.Hostname
		}
	}
	_ = db.DB.Save(&agent)

	ac.AgentID = agent.ID
	h.Register(agent.ID, ac)

	_ = ac.QueueEnvelope(protocol.TypeRegisterResponse, protocol.RegisterResponse{
		Success: true,
		AgentID: agent.ID,
		Message: "Authentication successful",
	})

	h.SyncSingleAgent(agent.ID)

	go ac.writePump()
	go ac.readPump()
}

func writeDirectEnvelope(conn *websocket.Conn, msgType protocol.MessageType, payload any) error {
	env := protocol.Envelope{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, b)
}
