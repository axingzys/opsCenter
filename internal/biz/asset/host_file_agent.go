package asset

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
	sshclient "github.com/ydcloud-dy/opshub/pkg/ssh"
)

const (
	hostArchiveMaxFiles = 5000
	hostArchiveMaxDepth = 16
	hostArchiveMaxBytes = int64(2 * 1024 * 1024 * 1024) // 2 GiB
)

type hostArchiveState struct {
	fileCount int
	totalSize int64
}

func newHostArchiveState() *hostArchiveState {
	return &hostArchiveState{}
}

func (s *hostArchiveState) checkDepth(depth int) error {
	if depth > hostArchiveMaxDepth {
		return fmt.Errorf("目录层级过深，超过限制（最大 %d 层）", hostArchiveMaxDepth)
	}
	return nil
}

func (s *hostArchiveState) addFile(size int64) error {
	if s.fileCount+1 > hostArchiveMaxFiles {
		return fmt.Errorf("目录文件数量过多，超过限制（最大 %d 个）", hostArchiveMaxFiles)
	}
	if size < 0 {
		size = 0
	}
	nextTotal := s.totalSize + size
	if nextTotal > hostArchiveMaxBytes {
		return fmt.Errorf("目录总大小超过限制（最大 %s）", formatBinarySize(hostArchiveMaxBytes))
	}
	s.fileCount++
	s.totalSize = nextTotal
	return nil
}

func (uc *HostUseCase) shouldUseAgentFileProxy(host *Host) bool {
	return host != nil && host.OSType == OSTypeWindows && uc.effectiveManagementMode(host) == ManagementModeAgent
}

func (uc *HostUseCase) listFilesViaSSH(ctx context.Context, host *Host, remotePath string) ([]*HostFileEntry, error) {
	sshClient, displayPath, actualPath, err := uc.openSSHFileSession(ctx, host, remotePath)
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()

	statInfo, err := sshClient.StatFile(actualPath)
	if err != nil {
		return nil, fmt.Errorf("路径不存在或无权限访问: %s, 错误: %w", actualPath, err)
	}
	if !statInfo.IsDir {
		return nil, fmt.Errorf("路径不是目录: %s", displayPath)
	}

	files, err := sshClient.ListDir(actualPath)
	if err != nil {
		return nil, fmt.Errorf("列出目录失败: %w", err)
	}

	entries := make([]*HostFileEntry, 0, len(files))
	for _, item := range files {
		entries = append(entries, &HostFileEntry{
			Name:    item.Name,
			Path:    joinLinuxDisplayPath(displayPath, item.Name),
			Size:    item.Size,
			Mode:    item.Mode,
			IsDir:   item.IsDir,
			ModTime: item.ModTime,
		})
	}

	return entries, nil
}

func (uc *HostUseCase) uploadFileViaSSH(ctx context.Context, host *Host, reader io.Reader, remotePath, filename string) error {
	sshClient, _, actualPath, err := uc.openSSHFileSession(ctx, host, remotePath)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	fullPath := pathpkg.Join(actualPath, filename)
	if err := sshClient.UploadFromReader(reader, fullPath); err != nil {
		return fmt.Errorf("上传文件失败: %w", err)
	}

	return nil
}

func (uc *HostUseCase) downloadFileViaSSH(ctx context.Context, host *Host, remotePath string, writer io.Writer) error {
	sshClient, _, actualPath, err := uc.openSSHFileSession(ctx, host, remotePath)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	statInfo, err := sshClient.StatFile(actualPath)
	if err != nil {
		return fmt.Errorf("读取文件信息失败: %w", err)
	}

	if statInfo.IsDir {
		return uc.downloadDirectoryViaSSH(ctx, sshClient, actualPath, remotePath, writer)
	}

	if err := sshClient.DownloadToWriter(actualPath, writer); err != nil {
		return fmt.Errorf("下载文件失败: %w", err)
	}

	return nil
}

func (uc *HostUseCase) downloadDirectoryViaSSH(ctx context.Context, sshClient *sshclient.Client, actualPath, remotePath string, writer io.Writer) error {
	sftpClient, err := sshClient.NewSFTPClient()
	if err != nil {
		return fmt.Errorf("创建 SFTP 客户端失败: %w", err)
	}
	defer sftpClient.Close()

	zipWriter := zip.NewWriter(writer)
	rootName := deriveDirectoryArchiveBaseName(remotePath, actualPath)
	archiveState := newHostArchiveState()
	if err := uc.addSSHDirectoryToZip(ctx, sftpClient, actualPath, rootName, 0, archiveState, zipWriter); err != nil {
		_ = zipWriter.Close()
		return err
	}
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("生成压缩文件失败: %w", err)
	}

	return nil
}

func (uc *HostUseCase) addSSHDirectoryToZip(ctx context.Context, sftpClient *sftp.Client, remotePath, archivePath string, depth int, archiveState *hostArchiveState, zipWriter *zip.Writer) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("下载已取消: %w", err)
	}
	if err := archiveState.checkDepth(depth); err != nil {
		return err
	}

	items, err := sftpClient.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("读取目录失败: %s, 错误: %w", remotePath, err)
	}

	if len(items) == 0 {
		if err := addEmptyDirectoryToZip(zipWriter, archivePath); err != nil {
			return fmt.Errorf("写入空目录失败: %s, 错误: %w", archivePath, err)
		}
		return nil
	}

	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("下载已取消: %w", err)
		}

		entryName := sanitizeArchivePathSegment(item.Name())
		if entryName == "" {
			continue
		}

		childRemotePath := pathpkg.Join(remotePath, item.Name())
		childArchivePath := pathpkg.Join(archivePath, entryName)

		if item.IsDir() {
			if err := uc.addSSHDirectoryToZip(ctx, sftpClient, childRemotePath, childArchivePath, depth+1, archiveState, zipWriter); err != nil {
				return err
			}
			continue
		}

		if err := archiveState.addFile(item.Size()); err != nil {
			return err
		}

		if err := addSSHFileToZip(sftpClient, childRemotePath, childArchivePath, item, zipWriter); err != nil {
			return err
		}
	}

	return nil
}

func addSSHFileToZip(sftpClient *sftp.Client, remotePath, archivePath string, info os.FileInfo, zipWriter *zip.Writer) error {
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("创建压缩条目失败: %w", err)
	}
	header.Name = archivePath
	header.Method = zip.Deflate

	entryWriter, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("创建压缩条目失败: %w", err)
	}

	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("打开远程文件失败: %s, 错误: %w", remotePath, err)
	}
	defer remoteFile.Close()

	if _, err := io.Copy(entryWriter, remoteFile); err != nil {
		return fmt.Errorf("写入压缩条目失败: %s, 错误: %w", archivePath, err)
	}

	return nil
}

func (uc *HostUseCase) deleteFileViaSSH(ctx context.Context, host *Host, remotePath string) error {
	sshClient, _, actualPath, err := uc.openSSHFileSession(ctx, host, remotePath)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	if err := sshClient.RemoveFile(actualPath); err != nil {
		return fmt.Errorf("删除文件失败: %w", err)
	}

	return nil
}

func (uc *HostUseCase) openSSHFileSession(ctx context.Context, host *Host, remotePath string) (*sshclient.Client, string, string, error) {
	if host.CredentialID == 0 {
		return nil, "", "", fmt.Errorf("主机未配置凭证")
	}

	credential, err := uc.credentialRepo.GetByIDDecrypted(ctx, host.CredentialID)
	if err != nil {
		return nil, "", "", fmt.Errorf("获取凭证失败: %w", err)
	}

	sshClient, err := uc.createSSHClient(host, credential)
	if err != nil {
		return nil, "", "", fmt.Errorf("创建SSH连接失败: %w", err)
	}

	displayPath, actualPath, err := resolveLinuxRemotePath(sshClient, remotePath)
	if err != nil {
		sshClient.Close()
		return nil, "", "", err
	}

	return sshClient, displayPath, actualPath, nil
}

func resolveLinuxRemotePath(sshClient *sshclient.Client, remotePath string) (string, string, error) {
	displayPath := strings.TrimSpace(remotePath)
	if displayPath == "" {
		displayPath = "~"
	}

	actualPath := displayPath
	if strings.HasPrefix(displayPath, "~") {
		homeDir, err := sshClient.Execute("echo $HOME")
		if err != nil {
			return "", "", fmt.Errorf("获取用户主目录失败: %w", err)
		}
		actualPath = strings.Replace(displayPath, "~", strings.TrimSpace(homeDir), 1)
	}

	return displayPath, actualPath, nil
}

func joinLinuxDisplayPath(basePath, name string) string {
	basePath = strings.TrimSpace(basePath)
	switch basePath {
	case "", "~":
		return "~/" + name
	case "/":
		return "/" + name
	default:
		return strings.TrimRight(basePath, "/") + "/" + name
	}
}

func (uc *HostUseCase) listFilesViaAgent(ctx context.Context, host *Host, remotePath string) ([]*HostFileEntry, error) {
	endpoint, token, err := uc.resolveAgentFileProxy(ctx, host)
	if err != nil {
		return nil, err
	}

	return uc.listFilesViaAgentWithAuth(ctx, endpoint, token, remotePath)
}

func (uc *HostUseCase) listFilesViaAgentWithAuth(ctx context.Context, endpoint, token, remotePath string) ([]*HostFileEntry, error) {
	reqURL, err := buildAgentFileURL(endpoint, "/files", strings.TrimSpace(remotePath))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Agent 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := newAgentFileHTTPClient(45 * time.Second).Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Agent 文件列表失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求 Agent 文件列表失败: %s", readAgentFileError(resp.Body))
	}

	var entries []*HostFileEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("解析 Agent 文件列表失败: %w", err)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries, nil
}

func (uc *HostUseCase) uploadFileViaAgent(ctx context.Context, host *Host, reader io.Reader, remotePath, filename string) error {
	endpoint, token, err := uc.resolveAgentFileProxy(ctx, host)
	if err != nil {
		return err
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		if err := writer.WriteField("path", strings.TrimSpace(remotePath)); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		if _, err := io.Copy(part, reader); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/files/upload", pr)
	if err != nil {
		return fmt.Errorf("创建 Agent 上传请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := newAgentFileHTTPClient(30 * time.Minute).Do(req)
	if err != nil {
		return fmt.Errorf("上传文件到 Agent 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传文件到 Agent 失败: %s", readAgentFileError(resp.Body))
	}

	return nil
}

func (uc *HostUseCase) downloadFileViaAgent(ctx context.Context, host *Host, remotePath string, writer io.Writer) error {
	endpoint, token, err := uc.resolveAgentFileProxy(ctx, host)
	if err != nil {
		return err
	}

	normalizedPath := normalizeWindowsAgentPath(remotePath)
	if normalizedPath == "" {
		return fmt.Errorf("请指定具体路径")
	}

	_, listErr := uc.listFilesViaAgentWithAuth(ctx, endpoint, token, normalizedPath)
	if listErr == nil {
		return uc.downloadDirectoryViaAgent(ctx, endpoint, token, normalizedPath, writer)
	}
	if !isAgentNotDirectoryError(listErr) {
		return listErr
	}

	return uc.downloadFileViaAgentWithAuth(ctx, endpoint, token, normalizedPath, writer)
}

func (uc *HostUseCase) downloadDirectoryViaAgent(ctx context.Context, endpoint, token, remotePath string, writer io.Writer) error {
	zipWriter := zip.NewWriter(writer)
	rootName := deriveDirectoryArchiveBaseName(remotePath, remotePath)
	archiveState := newHostArchiveState()

	if err := uc.addAgentDirectoryToZip(ctx, endpoint, token, remotePath, rootName, 0, archiveState, zipWriter); err != nil {
		_ = zipWriter.Close()
		return err
	}
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("生成压缩文件失败: %w", err)
	}

	return nil
}

func (uc *HostUseCase) addAgentDirectoryToZip(ctx context.Context, endpoint, token, remotePath, archivePath string, depth int, archiveState *hostArchiveState, zipWriter *zip.Writer) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("下载已取消: %w", err)
	}
	if err := archiveState.checkDepth(depth); err != nil {
		return err
	}

	entries, err := uc.listFilesViaAgentWithAuth(ctx, endpoint, token, remotePath)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		if err := addEmptyDirectoryToZip(zipWriter, archivePath); err != nil {
			return fmt.Errorf("写入空目录失败: %s, 错误: %w", archivePath, err)
		}
		return nil
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("下载已取消: %w", err)
		}

		entryName := sanitizeArchivePathSegment(entry.Name)
		if entryName == "" {
			continue
		}

		childRemotePath := strings.TrimSpace(entry.Path)
		if childRemotePath == "" {
			childRemotePath = joinWindowsRemotePath(remotePath, entry.Name)
		}
		childArchivePath := pathpkg.Join(archivePath, entryName)

		if entry.IsDir {
			if err := uc.addAgentDirectoryToZip(ctx, endpoint, token, childRemotePath, childArchivePath, depth+1, archiveState, zipWriter); err != nil {
				return err
			}
			continue
		}

		if err := archiveState.addFile(entry.Size); err != nil {
			return err
		}

		if err := uc.addAgentFileToZip(ctx, endpoint, token, childRemotePath, childArchivePath, entry, zipWriter); err != nil {
			return err
		}
	}

	return nil
}

func (uc *HostUseCase) addAgentFileToZip(ctx context.Context, endpoint, token, remotePath, archivePath string, entry *HostFileEntry, zipWriter *zip.Writer) error {
	reader, err := uc.openAgentDownloadStream(ctx, endpoint, token, remotePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	header := &zip.FileHeader{
		Name:   archivePath,
		Method: zip.Deflate,
	}
	if entry != nil {
		if entry.Size > 0 {
			header.UncompressedSize64 = uint64(entry.Size)
		}
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(entry.ModTime), time.Local); err == nil {
			header.Modified = t
		}
	}

	entryWriter, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("创建压缩条目失败: %w", err)
	}

	if _, err := io.Copy(entryWriter, reader); err != nil {
		return fmt.Errorf("写入压缩条目失败: %s, 错误: %w", archivePath, err)
	}

	return nil
}

func (uc *HostUseCase) downloadFileViaAgentWithAuth(ctx context.Context, endpoint, token, remotePath string, writer io.Writer) error {
	reader, err := uc.openAgentDownloadStream(ctx, endpoint, token, remotePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if _, err := io.Copy(writer, reader); err != nil {
		return fmt.Errorf("读取 Agent 文件内容失败: %w", err)
	}

	return nil
}

func (uc *HostUseCase) openAgentDownloadStream(ctx context.Context, endpoint, token, remotePath string) (io.ReadCloser, error) {
	reqURL, err := buildAgentFileURL(endpoint, "/files/download", strings.TrimSpace(remotePath))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Agent 下载请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := newAgentFileHTTPClient(30 * time.Minute).Do(req)
	if err != nil {
		return nil, fmt.Errorf("从 Agent 下载文件失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("从 Agent 下载文件失败: %s", readAgentFileError(resp.Body))
	}

	return resp.Body, nil
}

func (uc *HostUseCase) deleteFileViaAgent(ctx context.Context, host *Host, remotePath string) error {
	endpoint, token, err := uc.resolveAgentFileProxy(ctx, host)
	if err != nil {
		return err
	}

	reqURL, err := buildAgentFileURL(endpoint, "/files", strings.TrimSpace(remotePath))
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("创建 Agent 删除请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := newAgentFileHTTPClient(45 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("删除 Agent 文件失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("删除 Agent 文件失败: %s", readAgentFileError(resp.Body))
	}

	return nil
}

func (uc *HostUseCase) resolveAgentFileProxy(ctx context.Context, host *Host) (string, string, error) {
	return uc.resolveAgentEndpoint(ctx, host)
}

func buildAgentFileURL(endpoint, routePath, filePath string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(endpoint, "/") + routePath)
	if err != nil {
		return "", fmt.Errorf("构造 Agent URL 失败: %w", err)
	}
	if filePath != "" {
		query := parsed.Query()
		query.Set("path", filePath)
		parsed.RawQuery = query.Encode()
	}
	return parsed.String(), nil
}

func readAgentFileError(reader io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(reader, 4096))
	if err != nil {
		return "读取错误信息失败"
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		return "Agent 未返回错误信息"
	}
	return message
}

func newAgentFileHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) {
			return nil, nil
		},
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func formatBinarySize(size int64) string {
	if size <= 0 {
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(size)
	unitIndex := 0
	for value >= 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}

	if unitIndex == 0 {
		return fmt.Sprintf("%d %s", size, units[unitIndex])
	}
	return fmt.Sprintf("%.2f %s", value, units[unitIndex])
}

func normalizeWindowsAgentPath(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "/", `\`))
	if path == "" {
		return ""
	}
	if len(path) == 2 && path[1] == ':' {
		return path + `\`
	}
	if len(path) >= 3 && path[1] == ':' && path[2] != '\\' {
		return path[:2] + `\` + path[2:]
	}
	return filepath.Clean(path)
}

func joinWindowsRemotePath(basePath, name string) string {
	basePath = normalizeWindowsAgentPath(basePath)
	if basePath == "" {
		return normalizeWindowsAgentPath(name)
	}
	if strings.HasSuffix(basePath, `\`) {
		return basePath + name
	}
	return basePath + `\` + name
}

func deriveDirectoryArchiveBaseName(remotePath, fallbackPath string) string {
	baseName := sanitizeArchivePathSegment(pathBaseName(remotePath))
	if baseName == "" {
		baseName = sanitizeArchivePathSegment(pathBaseName(fallbackPath))
	}
	if baseName == "" {
		baseName = "directory"
	}
	return baseName
}

func pathBaseName(remotePath string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(remotePath, "\\", "/"))
	normalized = strings.TrimRight(normalized, "/")
	if normalized == "" || normalized == "~" {
		return ""
	}
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		return normalized[idx+1:]
	}
	return normalized
}

func sanitizeArchivePathSegment(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, `\`, "_"))
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, ":", "")
	switch name {
	case "", ".", "..":
		return ""
	default:
		return name
	}
}

func addEmptyDirectoryToZip(zipWriter *zip.Writer, archivePath string) error {
	archivePath = strings.TrimSuffix(archivePath, "/") + "/"
	header := &zip.FileHeader{
		Name:   archivePath,
		Method: zip.Store,
	}
	header.SetMode(os.ModeDir | 0o755)
	_, err := zipWriter.CreateHeader(header)
	return err
}

func isAgentNotDirectoryError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "路径不是目录") || strings.Contains(strings.ToLower(message), "not a directory")
}
