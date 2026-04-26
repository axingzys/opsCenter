package asset

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ydcloud-dy/opshub/internal/conf"
	guac "github.com/ydcloud-dy/opshub/pkg/desktop/guacamole"
)

type DesktopSessionUseCase struct {
	hostRepo       HostRepo
	credentialRepo CredentialRepo
	sessionRepo    DesktopSessionRepo
	cfg            conf.DesktopConfig
}

func NewDesktopSessionUseCase(hostRepo HostRepo, credentialRepo CredentialRepo, sessionRepo DesktopSessionRepo, cfg conf.DesktopConfig) *DesktopSessionUseCase {
	return &DesktopSessionUseCase{
		hostRepo:       hostRepo,
		credentialRepo: credentialRepo,
		sessionRepo:    sessionRepo,
		cfg:            cfg,
	}
}

func (uc *DesktopSessionUseCase) CreateLaunch(ctx context.Context, hostID, userID uint, username, clientIP string, req *DesktopLaunchRequest) (*DesktopLaunchResponse, error) {
	if !uc.cfg.Enabled {
		return nil, fmt.Errorf("远程桌面功能未启用")
	}
	if uc.cfg.Provider != "" && uc.cfg.Provider != "guacamole" {
		return nil, fmt.Errorf("暂不支持桌面网关: %s", uc.cfg.Provider)
	}
	if uc.cfg.JSONSecretKey == "" {
		return nil, fmt.Errorf("未配置 guacamole json secret key")
	}

	host, err := uc.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return nil, fmt.Errorf("主机不存在")
	}
	if host.OSType != "windows" {
		return nil, fmt.Errorf("仅支持对 Windows 主机发起桌面连接")
	}
	if !host.DesktopEnabled {
		return nil, fmt.Errorf("该主机未启用桌面访问")
	}
	if host.DesktopProtocol != "" && host.DesktopProtocol != "rdp" {
		return nil, fmt.Errorf("暂不支持桌面协议: %s", host.DesktopProtocol)
	}
	if host.DesktopCredentialID == 0 {
		return nil, fmt.Errorf("主机未配置桌面凭据")
	}

	credential, err := uc.credentialRepo.GetByIDDecrypted(ctx, host.DesktopCredentialID)
	if err != nil {
		return nil, fmt.Errorf("获取桌面凭据失败: %w", err)
	}
	if credential.Protocol != "" && credential.Protocol != "rdp" {
		return nil, fmt.Errorf("桌面凭据协议必须为 RDP")
	}
	if credential.Type != "password" {
		return nil, fmt.Errorf("RDP 桌面仅支持密码凭据")
	}
	if credential.Username == "" || credential.Password == "" {
		return nil, fmt.Errorf("桌面凭据不完整")
	}

	sessionUUID := uuid.NewString()
	now := time.Now()
	recordingPath := ""
	recordingName := ""
	if uc.cfg.RecordingPath != "" {
		recordingName = sessionUUID + ".guac"
		recordingPath = path.Join(uc.cfg.RecordingPath, recordingName)
	}

	resolution := ""
	if req != nil && req.Width > 0 && req.Height > 0 {
		resolution = fmt.Sprintf("%dx%d", req.Width, req.Height)
	}

	session := &DesktopSession{
		SessionUUID:   sessionUUID,
		HostID:        host.ID,
		HostName:      host.Name,
		HostIP:        host.IP,
		UserID:        userID,
		Username:      username,
		Provider:      "guacamole",
		Protocol:      "rdp",
		Status:        "active",
		ClientIP:      clientIP,
		Resolution:    resolution,
		RecordingPath: recordingPath,
		StartedAt:     &now,
	}
	if err := uc.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("创建桌面会话失败: %w", err)
	}

	ttlSeconds := uc.cfg.TokenTTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = 30
	}
	port := host.DesktopPort
	if port == 0 {
		port = 3389
	}
	security := host.DesktopSecurity
	if security == "" {
		security = "nla"
	}

	params := map[string]string{
		"hostname":         host.IP,
		"port":             strconv.Itoa(port),
		"username":         credential.Username,
		"password":         credential.Password,
		"security":         security,
		"ignore-cert":      strconv.FormatBool(host.DesktopIgnoreCert),
		"resize-method":    "display-update",
		"enable-wallpaper": "false",
		"enable-theming":   "false",
		"disable-audio":    "true",
	}
	if credential.Domain != "" {
		params["domain"] = credential.Domain
	}
	if req != nil {
		if req.Width > 0 {
			params["width"] = strconv.Itoa(req.Width)
		}
		if req.Height > 0 {
			params["height"] = strconv.Itoa(req.Height)
		}
		if req.DPI > 0 {
			params["dpi"] = strconv.Itoa(req.DPI)
		}
	}
	if uc.cfg.RecordingPath != "" {
		params["recording-path"] = uc.cfg.RecordingPath
		params["recording-name"] = recordingName
	}
	if uc.cfg.TransferRootPath != "" {
		params["enable-drive"] = "true"
		params["drive-name"] = uc.getDriveName()
		params["drive-path"] = path.Join(uc.cfg.TransferRootPath, "drive-"+sessionUUID)
		params["create-drive-path"] = "true"
		params["disable-upload"] = strconv.FormatBool(uc.cfg.DisableUpload)
		params["disable-download"] = strconv.FormatBool(uc.cfg.DisableDownload)
	}

	payload := &guac.AuthPayload{
		Username: username,
		Expires:  time.Now().Add(time.Duration(ttlSeconds) * time.Second).UnixMilli(),
		Connections: map[string]guac.Connection{
			host.Name: {
				ID:         sessionUUID,
				Protocol:   "rdp",
				Parameters: params,
			},
		},
	}

	data, err := guac.EncryptAuthPayload(uc.cfg.JSONSecretKey, payload)
	if err != nil {
		session.Status = "failed"
		session.CloseReason = err.Error()
		_ = uc.sessionRepo.Update(ctx, session)
		return nil, fmt.Errorf("生成桌面启动参数失败: %w", err)
	}

	launchURL := buildLaunchURL(uc.cfg.GetPublicPath(), data, host.Name)

	return &DesktopLaunchResponse{
		SessionID:   session.ID,
		SessionUUID: session.SessionUUID,
		LaunchURL:   launchURL,
		Status:      session.Status,
	}, nil
}

func (uc *DesktopSessionUseCase) getDriveName() string {
	name := strings.TrimSpace(uc.cfg.DriveName)
	if name == "" {
		return "OpsHub Files"
	}
	return name
}

func (uc *DesktopSessionUseCase) UploadFile(ctx context.Context, id, userID uint, reader io.Reader, filename string) (string, error) {
	if strings.TrimSpace(uc.cfg.TransferRootPath) == "" {
		return "", fmt.Errorf("当前桌面会话未启用文件映射")
	}

	session, err := uc.sessionRepo.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("桌面会话不存在")
	}
	if session.UserID != userID {
		return "", fmt.Errorf("无权操作该桌面会话")
	}
	if session.Status != "active" {
		return "", fmt.Errorf("桌面会话已关闭，无法上传文件")
	}

	safeName, err := sanitizeDesktopUploadFilename(filename)
	if err != nil {
		return "", err
	}

	drivePath := uc.getDrivePath(session.SessionUUID)
	if err := os.MkdirAll(drivePath, 0o755); err != nil {
		return "", fmt.Errorf("初始化桌面映射目录失败: %w", err)
	}

	targetPath := filepath.Join(drivePath, safeName)
	file, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return "", fmt.Errorf("写入目标文件失败: %w", err)
	}

	return safeName, nil
}

func (uc *DesktopSessionUseCase) getDrivePath(sessionUUID string) string {
	return filepath.Join(filepath.Clean(uc.cfg.TransferRootPath), "drive-"+sessionUUID)
}

func sanitizeDesktopUploadFilename(filename string) (string, error) {
	name := strings.TrimSpace(strings.ReplaceAll(filename, "\x00", ""))
	if name == "" {
		return "", fmt.Errorf("文件名不能为空")
	}

	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("文件名无效")
	}

	return name, nil
}

func (uc *DesktopSessionUseCase) List(ctx context.Context, page, pageSize int, keyword, status string, userID uint) ([]*DesktopSessionInfo, int64, error) {
	list, total, err := uc.sessionRepo.List(ctx, page, pageSize, keyword, status, userID)
	if err != nil {
		return nil, 0, err
	}

	out := make([]*DesktopSessionInfo, 0, len(list))
	for _, session := range list {
		out = append(out, toDesktopSessionInfo(session))
	}

	return out, total, nil
}

func (uc *DesktopSessionUseCase) ListByUserID(ctx context.Context, userID uint, page, pageSize int) ([]*DesktopSessionInfo, int64, error) {
	return uc.List(ctx, page, pageSize, "", "", userID)
}

func (uc *DesktopSessionUseCase) GetByID(ctx context.Context, id, userID uint) (*DesktopSessionInfo, error) {
	session, err := uc.sessionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, fmt.Errorf("无权访问该桌面会话")
	}
	return toDesktopSessionInfo(session), nil
}

func (uc *DesktopSessionUseCase) Close(ctx context.Context, id, userID uint, reason string) error {
	session, err := uc.sessionRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if session.UserID != userID {
		return fmt.Errorf("无权关闭该桌面会话")
	}
	if session.Status == "closed" {
		return nil
	}

	now := time.Now()
	session.Status = "closed"
	session.CloseReason = reason
	session.EndedAt = &now
	return uc.sessionRepo.Update(ctx, session)
}

func (uc *DesktopSessionUseCase) Delete(ctx context.Context, id uint) error {
	session, err := uc.sessionRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	recordingPath := filepath.Clean(strings.TrimSpace(session.RecordingPath))
	if recordingPath != "" && recordingPath != "." {
		if err := os.Remove(recordingPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除录屏文件失败: %w", err)
		}
	}

	if strings.TrimSpace(uc.cfg.TransferRootPath) != "" {
		drivePath := strings.TrimSpace(uc.getDrivePath(session.SessionUUID))
		if drivePath != "" && drivePath != "." {
			if err := os.RemoveAll(drivePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("删除桌面映射目录失败: %w", err)
			}
		}
	}

	if err := uc.sessionRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除桌面会话失败: %w", err)
	}

	return nil
}

func (uc *DesktopSessionUseCase) GetRecordingFile(ctx context.Context, id uint) (string, string, error) {
	session, err := uc.sessionRepo.GetByID(ctx, id)
	if err != nil {
		return "", "", err
	}

	recordingPath := filepath.Clean(strings.TrimSpace(session.RecordingPath))
	if recordingPath == "" || recordingPath == "." {
		return "", "", fmt.Errorf("该桌面会话没有录屏文件")
	}

	if uc.cfg.RecordingPath != "" {
		baseDir := filepath.Clean(uc.cfg.RecordingPath)
		relPath, err := filepath.Rel(baseDir, recordingPath)
		if err != nil {
			return "", "", fmt.Errorf("录屏文件路径无效")
		}
		if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
			return "", "", fmt.Errorf("录屏文件路径无效")
		}
	}

	fileInfo, err := os.Stat(recordingPath)
	if err != nil {
		return "", "", fmt.Errorf("录屏文件不存在: %w", err)
	}
	if fileInfo.IsDir() {
		return "", "", fmt.Errorf("录屏文件路径无效")
	}

	return recordingPath, filepath.Base(recordingPath), nil
}

func toDesktopSessionInfo(session *DesktopSession) *DesktopSessionInfo {
	info := &DesktopSessionInfo{
		ID:            session.ID,
		SessionUUID:   session.SessionUUID,
		HostID:        session.HostID,
		HostName:      session.HostName,
		HostIP:        session.HostIP,
		UserID:        session.UserID,
		Username:      session.Username,
		Provider:      session.Provider,
		Protocol:      session.Protocol,
		Status:        session.Status,
		ClientIP:      session.ClientIP,
		Resolution:    session.Resolution,
		RecordingPath: session.RecordingPath,
		CreateTime:    session.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdateTime:    session.UpdatedAt.Format("2006-01-02 15:04:05"),
		CloseReason:   session.CloseReason,
	}
	if session.StartedAt != nil {
		info.StartedAt = session.StartedAt.Format("2006-01-02 15:04:05")
	}
	if session.EndedAt != nil {
		info.EndedAt = session.EndedAt.Format("2006-01-02 15:04:05")
	}
	info.DurationSeconds = calculateDesktopSessionDurationSeconds(session)
	info.RecordingAvailable, info.FileSize = getDesktopSessionRecordingMeta(session)
	return info
}

func calculateDesktopSessionDurationSeconds(session *DesktopSession) int64 {
	if session.StartedAt == nil {
		return 0
	}

	endTime := time.Now()
	if session.EndedAt != nil && !session.EndedAt.IsZero() {
		endTime = *session.EndedAt
	}

	duration := endTime.Sub(*session.StartedAt)
	if duration < 0 {
		return 0
	}

	return int64(duration.Seconds())
}

func getDesktopSessionRecordingMeta(session *DesktopSession) (bool, int64) {
	recordingPath := filepath.Clean(strings.TrimSpace(session.RecordingPath))
	if recordingPath == "" || recordingPath == "." {
		return false, 0
	}

	fileInfo, err := os.Stat(recordingPath)
	if err != nil || fileInfo.IsDir() {
		return false, 0
	}

	return true, fileInfo.Size()
}

func buildLaunchURL(publicPath, data, connectionID string) string {
	pathValue := strings.TrimSpace(publicPath)
	if pathValue == "" {
		pathValue = "/guacamole/"
	}
	separator := "?"
	if strings.Contains(pathValue, "?") {
		separator = "&"
	}

	launchURL := pathValue + separator + "data=" + url.QueryEscape(data)
	if strings.TrimSpace(connectionID) == "" {
		return launchURL
	}

	return launchURL + "#/client/" + buildGuacamoleClientIdentifier(connectionID, "json")
}

func buildGuacamoleClientIdentifier(connectionID, dataSource string) string {
	raw := strings.Join([]string{
		connectionID,
		"c",
		dataSource,
	}, "\x00")
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}
