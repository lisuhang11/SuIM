package api

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"net/http"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

const agentVoiceKeyPrefix = "suim:agent:voice:"

// suimVoice is a SuIM-owned catalog entry (edge-tts short name as id).
type suimVoice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Language string `json:"language"`
	Gender   string `json:"gender"`
	Provider string `json:"provider"`
}

var suimVoiceCatalog = []suimVoice{
	{ID: "zh-CN-XiaoxiaoNeural", Name: "晓晓", Language: "中文", Gender: "女", Provider: "edge-tts"},
	{ID: "zh-CN-XiaoyiNeural", Name: "晓伊", Language: "中文", Gender: "女", Provider: "edge-tts"},
	{ID: "zh-CN-XiaohanNeural", Name: "晓涵", Language: "中文", Gender: "女", Provider: "edge-tts"},
	{ID: "zh-CN-XiaomengNeural", Name: "晓梦", Language: "中文", Gender: "女", Provider: "edge-tts"},
	{ID: "zh-CN-XiaomoNeural", Name: "晓墨", Language: "中文", Gender: "女", Provider: "edge-tts"},
	{ID: "zh-CN-XiaoqiuNeural", Name: "晓秋", Language: "中文", Gender: "女", Provider: "edge-tts"},
	{ID: "zh-CN-XiaoruiNeural", Name: "晓睿", Language: "中文", Gender: "女", Provider: "edge-tts"},
	{ID: "zh-CN-XiaoxuanNeural", Name: "晓萱", Language: "中文", Gender: "女", Provider: "edge-tts"},
	{ID: "zh-CN-XiaozhenNeural", Name: "晓甄", Language: "中文", Gender: "女", Provider: "edge-tts"},
	{ID: "zh-CN-XiaoshuangNeural", Name: "晓双", Language: "中文", Gender: "女童", Provider: "edge-tts"},
	{ID: "zh-CN-YunxiNeural", Name: "云希", Language: "中文", Gender: "男", Provider: "edge-tts"},
	{ID: "zh-CN-YunyangNeural", Name: "云扬", Language: "中文", Gender: "男", Provider: "edge-tts"},
	{ID: "zh-CN-YunjianNeural", Name: "云健", Language: "中文", Gender: "男", Provider: "edge-tts"},
	{ID: "zh-CN-YunfengNeural", Name: "云枫", Language: "中文", Gender: "男", Provider: "edge-tts"},
	{ID: "zh-CN-YunhaoNeural", Name: "云皓", Language: "中文", Gender: "男", Provider: "edge-tts"},
	{ID: "zh-CN-YunzeNeural", Name: "云泽", Language: "中文", Gender: "男", Provider: "edge-tts"},
	{ID: "zh-CN-YunxiaNeural", Name: "云夏", Language: "中文", Gender: "男童", Provider: "edge-tts"},
}

func defaultSuimVoiceID() string {
	if v := strings.TrimSpace(os.Getenv("AGENT_TTS_VOICE")); v != "" {
		return v
	}
	return "zh-CN-XiaoxiaoNeural"
}

func knownSuimVoice(id string) bool {
	id = strings.TrimSpace(id)
	for _, v := range suimVoiceCatalog {
		if v.ID == id {
			return true
		}
	}
	return false
}

func (s *Server) listAgentVoices(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]any{
		"object": "list",
		"data":   suimVoiceCatalog,
	})
}

func (s *Server) previewAgentVoice(w http.ResponseWriter, r *http.Request) {
	var in struct {
		VoiceID string `json:"voice_id"`
		Text    string `json:"text"`
	}
	if err := decode(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	voiceID := strings.TrimSpace(in.VoiceID)
	if voiceID == "" {
		voiceID = defaultSuimVoiceID()
	}
	if !knownSuimVoice(voiceID) {
		writeErr(w, errorcode.ErrThirdInvalidArgs.WithMsg("unknown voice_id"))
		return
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		text = "你好，我是 SuIM 语音助手，这是当前音色的试听。"
	}
	python, script := s.previewTools()
	if python == "" || script == "" {
		writeErr(w, errorcode.ErrInternal.WithMsg("preview tools are not configured"))
		return
	}
	tmp, err := os.CreateTemp("", "suim-voice-*.mp3")
	if err != nil {
		writeErr(w, errorcode.ErrInternal.WithMsg("create temp file failed"))
		return
	}
	outPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(outPath)

	cmd := exec.CommandContext(r.Context(), python, script, "--voice", voiceID, "--text", text, "--out", outPath)
	cmd.Dir = filepath.Dir(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeErr(w, errorcode.ErrInternal.WithMsg("preview synthesize failed: "+truncate(string(out), 200)))
		return
	}
	mp3, err := os.ReadFile(outPath)
	if err != nil || len(mp3) == 0 {
		writeErr(w, errorcode.ErrInternal.WithMsg("preview audio missing"))
		return
	}
	writeOK(w, map[string]any{
		"voice_id":  voiceID,
		"audio_url": "data:audio/mpeg;base64," + base64.StdEncoding.EncodeToString(mp3),
	})
}

func (s *Server) getAgentVoice(w http.ResponseWriter, r *http.Request) {
	userID := current(r).userID
	selected := ""
	if s.rdb != nil {
		selected, _ = s.rdb.Get(r.Context(), agentVoiceKeyPrefix+userID).Result()
		selected = strings.TrimSpace(selected)
	}
	if selected == "" || !knownSuimVoice(selected) {
		selected = defaultSuimVoiceID()
	}
	writeOK(w, map[string]any{
		"voice_id":         selected,
		"default_voice_id": defaultSuimVoiceID(),
		"provider":         "edge-tts",
	})
}

func (s *Server) setAgentVoice(w http.ResponseWriter, r *http.Request) {
	if s.rdb == nil {
		writeErr(w, errorcode.ErrInternal.WithMsg("redis is not configured on apigateway"))
		return
	}
	var in struct {
		VoiceID string `json:"voice_id"`
	}
	if err := decode(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	voiceID := strings.TrimSpace(in.VoiceID)
	if !knownSuimVoice(voiceID) {
		writeErr(w, errorcode.ErrThirdInvalidArgs.WithMsg("unknown voice_id"))
		return
	}
	userID := current(r).userID
	if err := s.rdb.Set(r.Context(), agentVoiceKeyPrefix+userID, voiceID, 0).Err(); err != nil {
		writeErr(w, errorcode.ErrInternal.WithMsg("save voice preference failed"))
		return
	}
	writeOK(w, map[string]any{"voice_id": voiceID, "provider": "edge-tts"})
}

func (s *Server) previewTools() (python, script string) {
	python = strings.TrimSpace(s.previewPython)
	script = strings.TrimSpace(s.previewScript)
	if python == "" {
		python = strings.TrimSpace(os.Getenv("SUIM_VOICE_PREVIEW_PYTHON"))
	}
	if script == "" {
		script = strings.TrimSpace(os.Getenv("SUIM_VOICE_PREVIEW_SCRIPT"))
	}
	if python == "" {
		python = "/home/lisuhang/SuIM/agents/voice-assistant/.venv/bin/python"
	}
	if script == "" {
		script = "/home/lisuhang/SuIM/agents/voice-assistant/preview_tts.py"
	}
	if _, err := os.Stat(python); err != nil {
		return "", ""
	}
	if _, err := os.Stat(script); err != nil {
		return "", ""
	}
	return python, script
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
