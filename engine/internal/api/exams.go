package api

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

const publicBaseURLKey = "public_base_url"

func (s *Server) listExams(w http.ResponseWriter, r *http.Request) {
	exams, err := s.store.ListExams(userFrom(r))
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"items": exams})
}

func parseTimeValue(raw string, fallback time.Time) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if fallback.IsZero() {
			return time.Time{}, false
		}
		return fallback, true
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, layout := range layouts {
		if layout == "2006-01-02T15:04:05" || layout == "2006-01-02T15:04" ||
			layout == "2006-01-02 15:04:05" || layout == "2006-01-02 15:04" {
			if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
				return t, true
			}
			continue
		}
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (s *Server) createExam(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title            string `json:"title"`
		BankID           int64  `json:"bankId"`
		Count            int    `json:"count"`
		TimeLimitSec     int    `json:"timeLimitSec"`
		TimeLimitMin     int    `json:"timeLimitMin"`
		OpenAt           string `json:"openAt"`
		CloseAt          string `json:"closeAt"`
		Password         string `json:"password"`
		ShuffleQuestions bool   `json:"shuffleQuestions"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if _, ok := s.bankFor(w, r, in.BankID, false); !ok {
		return
	}
	if in.TimeLimitSec == 0 && in.TimeLimitMin > 0 {
		in.TimeLimitSec = in.TimeLimitMin * 60
	}
	openAt, ok := parseTimeValue(in.OpenAt, time.Now())
	if !ok {
		response.BadRequest(w, "开始时间格式不正确")
		return
	}
	closeAt, ok := parseTimeValue(in.CloseAt, time.Time{})
	if !ok {
		response.BadRequest(w, "结束时间格式不正确")
		return
	}
	exam, err := s.store.CreateExam(store.CreateExamInput{
		Title:            in.Title,
		BankID:           in.BankID,
		Count:            in.Count,
		TimeLimitSec:     in.TimeLimitSec,
		OpenAt:           openAt,
		CloseAt:          closeAt,
		Password:         in.Password,
		ShuffleQuestions: in.ShuffleQuestions,
	}, userFrom(r).ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	response.OK(w, exam)
}

func (s *Server) getExam(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "考试编号无效")
		return
	}
	exam, err := s.store.GetExam(id)
	if err != nil {
		response.NotFound(w, "考试不存在")
		return
	}
	if !canManageExam(userFrom(r), exam.OwnerID) {
		response.Forbidden(w, "没有权限查看该考试")
		return
	}
	response.OK(w, exam)
}

func (s *Server) closeExam(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "考试编号无效")
		return
	}
	exam, err := s.store.GetExam(id)
	if err != nil {
		response.NotFound(w, "考试不存在")
		return
	}
	if !canManageExam(userFrom(r), exam.OwnerID) {
		response.Forbidden(w, "没有权限操作该考试")
		return
	}
	if err := s.store.CloseExam(id); err != nil {
		response.Internal(w, err)
		return
	}
	exam, err = s.store.GetExam(id)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, exam)
}

func (s *Server) deleteExam(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "考试编号无效")
		return
	}
	exam, err := s.store.GetExam(id)
	if err != nil {
		response.NotFound(w, "考试不存在")
		return
	}
	if !canManageExam(userFrom(r), exam.OwnerID) {
		response.Forbidden(w, "没有权限删除该考试")
		return
	}
	if err := s.store.DeleteExam(id); err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, nil)
}

func (s *Server) listExamAttempts(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "考试编号无效")
		return
	}
	exam, err := s.store.GetExam(id)
	if err != nil {
		response.NotFound(w, "考试不存在")
		return
	}
	if !canManageExam(userFrom(r), exam.OwnerID) {
		response.Forbidden(w, "没有权限查看该考试")
		return
	}
	items, err := s.store.ListExamAttempts(id)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"items": items})
}

func (s *Server) shareInfo(w http.ResponseWriter, r *http.Request) {
	publicBase, _ := s.store.GetSetting(publicBaseURLKey)
	response.OK(w, map[string]any{
		"port":          s.port,
		"lanAddresses":  lanIPv4(),
		"publicBaseUrl": publicBase,
		"linkPrefix":    "/e/",
	})
}

func (s *Server) updateShareSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PublicBaseURL string `json:"publicBaseUrl"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	raw := strings.TrimSpace(in.PublicBaseURL)
	if raw != "" {
		if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			response.BadRequest(w, "公网地址需以 http:// 或 https:// 开头")
			return
		}
		if _, err := url.Parse(raw); err != nil {
			response.BadRequest(w, "公网地址格式不正确")
			return
		}
		raw = strings.TrimRight(raw, "/")
	}
	if err := s.store.SetSetting(publicBaseURLKey, raw); err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"publicBaseUrl": raw})
}

// lanIPv4 返回可分享给同网段设备的地址：优先默认出口网卡，过滤 Hyper-V / VMware 等虚拟网卡。
func lanIPv4() []string {
	primary := outboundIPv4()
	type candidate struct {
		ip      string
		virtual bool
	}
	cands := []candidate{}
	seen := map[string]bool{}
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			name := strings.ToLower(iface.Name)
			virtual := containsAny(name, "vethernet", "vmware", "virtualbox", "hyper-v",
				"wsl", "docker", "bluetooth", "teredo", "isatap", "npcap", "loopback adapter")
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.To4() == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
					continue
				}
				s := ip.To4().String()
				if seen[s] {
					continue
				}
				seen[s] = true
				cands = append(cands, candidate{ip: s, virtual: virtual})
			}
		}
	}
	real := []string{}
	for _, c := range cands {
		if !c.virtual {
			real = append(real, c.ip)
		}
	}
	if len(real) == 0 {
		for _, c := range cands {
			real = append(real, c.ip)
		}
	}
	out := []string{}
	if primary != "" {
		out = append(out, primary)
	}
	for _, ip := range real {
		if ip != primary {
			out = append(out, ip)
		}
	}
	return out
}

func outboundIPv4() string {
	conn, err := net.DialTimeout("udp4", "223.5.5.5:80", 300*time.Millisecond)
	if err != nil {
		return ""
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP.To4() != nil {
		return addr.IP.To4().String()
	}
	return ""
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func (s *Server) publicExamInfo(w http.ResponseWriter, r *http.Request) {
	exam, err := s.store.PublicExamInfo(r.PathValue("token"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	response.OK(w, exam)
}

func (s *Server) publicStart(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name      string `json:"name"`
		StudentNo string `json:"studentNo"`
		Password  string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	start, err := s.store.StartExamAttempt(r.PathValue("token"), in.Name, in.StudentNo, in.Password)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	response.OK(w, start)
}

func (s *Server) publicSaveAnswer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AttemptToken string   `json:"attemptToken"`
		Seq          int      `json:"seq"`
		Chosen       []string `json:"chosen"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if in.Seq < 1 {
		response.BadRequest(w, "题号无效")
		return
	}
	if err := s.store.SaveExamAnswer(r.PathValue("token"), in.AttemptToken, in.Seq, in.Chosen); err != nil {
		writeStoreError(w, err)
		return
	}
	response.OK(w, nil)
}

func (s *Server) publicResume(w http.ResponseWriter, r *http.Request) {
	attemptToken := r.URL.Query().Get("attemptToken")
	if attemptToken == "" {
		response.BadRequest(w, "缺少答题凭证")
		return
	}
	start, err := s.store.ResumeExamAttempt(r.PathValue("token"), attemptToken)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	response.OK(w, start)
}

func (s *Server) publicSubmit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AttemptToken string `json:"attemptToken"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	result, err := s.store.SubmitExamAttempt(r.PathValue("token"), in.AttemptToken)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	response.OK(w, result)
}

func (s *Server) publicResult(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("attemptToken")
	if token == "" {
		response.BadRequest(w, "缺少答题凭证")
		return
	}
	result, err := s.store.AttemptResult(r.PathValue("token"), token)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	response.OK(w, result)
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrExamNotFound):
		response.NotFound(w, err.Error())
	case errors.Is(err, store.ErrExamClosed),
		errors.Is(err, store.ErrExamNotOpen),
		errors.Is(err, store.ErrWrongPassword),
		errors.Is(err, store.ErrAttemptExpired),
		errors.Is(err, store.ErrAttemptNotFound):
		response.BadRequest(w, err.Error())
	default:
		log.Printf("api error: %v", err)
		response.BadRequest(w, err.Error())
	}
}
