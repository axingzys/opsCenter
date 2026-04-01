package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/ydcloud-dy/opshub/pkg/logger"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/provider/dns"
	"go.uber.org/zap"
)

// CA Provider目录URL
const (
	// Let's Encrypt
	LEDirectoryProduction = "https://acme-v02.api.letsencrypt.org/directory"
	LEDirectoryStaging    = "https://acme-staging-v02.api.letsencrypt.org/directory"

	// ZeroSSL
	ZeroSSLDirectory = "https://acme.zerossl.com/v2/DV90"

	// Google Trust Services
	GoogleDirectory = "https://dv.acme-v02.api.pki.goog/directory"

	// BuyPass
	BuyPassDirectoryProduction = "https://api.buypass.com/acme/directory"
	BuyPassDirectoryStaging    = "https://api.test4.buypass.no/acme/directory"
)

// Client ACME客户端
type Client struct {
	email        string
	staging      bool
	caProvider   string
	keyAlgorithm string
	dnsProvider  dns.Provider
}

// User ACME用户
type User struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

// GetEmail 获取邮箱
func (u *User) GetEmail() string {
	return u.Email
}

// GetRegistration 获取注册信息
func (u *User) GetRegistration() *registration.Resource {
	return u.Registration
}

// GetPrivateKey 获取私钥
func (u *User) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// NewClient 创建ACME客户端
func NewClient(email string, staging bool, dnsProvider dns.Provider) *Client {
	return &Client{
		email:        email,
		staging:      staging,
		caProvider:   "letsencrypt", // 默认使用Let's Encrypt
		keyAlgorithm: "ec256",       // 默认使用EC256
		dnsProvider:  dnsProvider,
	}
}

// NewClientWithOptions 创建ACME客户端(带选项)
func NewClientWithOptions(email string, staging bool, caProvider, keyAlgorithm string, dnsProvider dns.Provider) *Client {
	if caProvider == "" {
		caProvider = "letsencrypt"
	}
	if keyAlgorithm == "" {
		keyAlgorithm = "ec256"
	}
	return &Client{
		email:        email,
		staging:      staging,
		caProvider:   caProvider,
		keyAlgorithm: keyAlgorithm,
		dnsProvider:  dnsProvider,
	}
}

// CertificateBundle 证书包
type CertificateBundle struct {
	Certificate string    // 证书PEM
	PrivateKey  string    // 私钥PEM
	IssuerCert  string    // 颁发者证书
	Domain      string    // 主域名
	SANDomains  []string  // SAN域名
	NotBefore   time.Time // 生效时间
	NotAfter    time.Time // 过期时间
	Fingerprint string    // 指纹
	Issuer      string    // 颁发者
}

// ObtainCertificate 申请证书
func (c *Client) ObtainCertificate(ctx context.Context, domains []string) (*CertificateBundle, error) {
	logger.Info("ACME: 开始申请证书", zap.Strings("domains", domains), zap.String("ca", c.caProvider))

	// 根据算法生成私钥
	privateKey, err := c.generatePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generate private key failed: %w", err)
	}
	logger.Info("ACME: 私钥生成完成")

	user := &User{
		Email: c.email,
		key:   privateKey,
	}

	// 创建ACME配置
	config := lego.NewConfig(user)
	config.CADirURL = c.getCADirectoryURL()
	config.Certificate.KeyType = c.getKeyType()

	logger.Info("ACME: 创建ACME客户端", zap.String("ca_url", config.CADirURL))

	// 创建ACME客户端
	client, err := lego.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create lego client failed: %w", err)
	}
	logger.Info("ACME: 客户端创建完成")

	// 设置DNS-01挑战提供者
	dnsChallenge := &DNSChallengeProvider{
		provider: c.dnsProvider,
		ctx:      ctx,
	}
	// 使用公共DNS服务器进行DNS传播验证，避免本地DNS缓存或超时问题
	// 设置较短的超时时间，加快失败检测
	err = client.Challenge.SetDNS01Provider(
		dnsChallenge,
		dns01.AddDNSTimeout(90*time.Second),
		dns01.DisableCompletePropagationRequirement(),
		dns01.AddRecursiveNameservers([]string{
			"8.8.8.8:53",      // Google DNS
			"8.8.4.4:53",      // Google DNS
			"1.1.1.1:53",      // Cloudflare DNS
			"223.5.5.5:53",    // 阿里云 DNS
			"119.29.29.29:53", // 腾讯云 DNS
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("set DNS-01 provider failed: %w", err)
	}
	logger.Info("ACME: DNS-01挑战提供者设置完成")

	// 注册用户
	logger.Info("ACME: 开始注册用户", zap.String("email", c.email))
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return nil, fmt.Errorf("register user failed: %w", err)
	}
	user.Registration = reg
	logger.Info("ACME: 用户注册完成")

	// 申请证书 - 使用goroutine包装，支持context取消
	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	type obtainResult struct {
		certs *certificate.Resource
		err   error
	}
	resultCh := make(chan obtainResult, 1)

	logger.Info("ACME: 开始获取证书（可能需要等待DNS验证）")
	go func() {
		certs, err := client.Certificate.Obtain(request)
		resultCh <- obtainResult{certs: certs, err: err}
	}()

	// 等待结果或context取消
	select {
	case <-ctx.Done():
		logger.Warn("ACME: 证书申请被取消或超时", zap.Error(ctx.Err()))
		return nil, fmt.Errorf("obtain certificate cancelled: %w", ctx.Err())
	case result := <-resultCh:
		if result.err != nil {
			logger.Error("ACME: 获取证书失败", zap.Error(result.err))
			return nil, fmt.Errorf("obtain certificate failed: %w", result.err)
		}

		logger.Info("ACME: 证书获取成功，正在解析")

		// 解析证书信息
		certInfo, err := parseCertificate(result.certs.Certificate)
		if err != nil {
			return nil, fmt.Errorf("parse certificate failed: %w", err)
		}

		logger.Info("ACME: 证书申请全部完成", zap.Strings("domains", domains))

		return &CertificateBundle{
			Certificate: string(result.certs.Certificate),
			PrivateKey:  string(result.certs.PrivateKey),
			IssuerCert:  string(result.certs.IssuerCertificate),
			Domain:      domains[0],
			SANDomains:  domains[1:],
			NotBefore:   certInfo.NotBefore,
			NotAfter:    certInfo.NotAfter,
			Fingerprint: certInfo.Fingerprint,
			Issuer:      certInfo.Issuer,
		}, nil
	}
}

// generatePrivateKey 根据算法生成私钥
func (c *Client) generatePrivateKey() (crypto.PrivateKey, error) {
	switch c.keyAlgorithm {
	case "rsa2048":
		return rsa.GenerateKey(rand.Reader, 2048)
	case "rsa3072":
		return rsa.GenerateKey(rand.Reader, 3072)
	case "rsa4096":
		return rsa.GenerateKey(rand.Reader, 4096)
	case "ec256":
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "ec384":
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	default:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
}

// getCADirectoryURL 获取CA目录URL
func (c *Client) getCADirectoryURL() string {
	switch c.caProvider {
	case "letsencrypt":
		if c.staging {
			return LEDirectoryStaging
		}
		return LEDirectoryProduction
	case "zerossl":
		return ZeroSSLDirectory
	case "google":
		return GoogleDirectory
	case "buypass":
		if c.staging {
			return BuyPassDirectoryStaging
		}
		return BuyPassDirectoryProduction
	default:
		if c.staging {
			return LEDirectoryStaging
		}
		return LEDirectoryProduction
	}
}

// getKeyType 获取密钥类型
func (c *Client) getKeyType() certcrypto.KeyType {
	switch c.keyAlgorithm {
	case "rsa2048":
		return certcrypto.RSA2048
	case "rsa3072":
		return certcrypto.RSA3072
	case "rsa4096":
		return certcrypto.RSA4096
	case "ec256":
		return certcrypto.EC256
	case "ec384":
		return certcrypto.EC384
	default:
		return certcrypto.EC256
	}
}

// DNSChallengeProvider DNS-01挑战提供者
type DNSChallengeProvider struct {
	provider dns.Provider
	ctx      context.Context
}

// Present 添加DNS TXT记录
func (d *DNSChallengeProvider) Present(domain, token, keyAuth string) error {
	fqdn, value := dns01.GetRecord(domain, keyAuth)
	// fqdn格式: _acme-challenge.example.com.
	// 去掉末尾的点
	if len(fqdn) > 0 && fqdn[len(fqdn)-1] == '.' {
		fqdn = fqdn[:len(fqdn)-1]
	}
	return d.provider.CreateTXTRecord(d.ctx, domain, fqdn, value)
}

// CleanUp 删除DNS TXT记录
func (d *DNSChallengeProvider) CleanUp(domain, token, keyAuth string) error {
	fqdn, _ := dns01.GetRecord(domain, keyAuth)
	if len(fqdn) > 0 && fqdn[len(fqdn)-1] == '.' {
		fqdn = fqdn[:len(fqdn)-1]
	}
	return d.provider.DeleteTXTRecord(d.ctx, domain, fqdn)
}

// CertInfo 证书信息
type CertInfo struct {
	NotBefore   time.Time
	NotAfter    time.Time
	Fingerprint string
	Issuer      string
	Subject     string
	SANs        []string
}

// parseCertificate 解析证书
func parseCertificate(certPEM []byte) (*CertInfo, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	fingerprint := fmt.Sprintf("%x", cert.Raw[:20])

	return &CertInfo{
		NotBefore:   cert.NotBefore,
		NotAfter:    cert.NotAfter,
		Fingerprint: fingerprint,
		Issuer:      cert.Issuer.String(),
		Subject:     cert.Subject.String(),
		SANs:        cert.DNSNames,
	}, nil
}

// ParseCertificatePEM 解析PEM证书
func ParseCertificatePEM(certPEM string) (*CertInfo, error) {
	return parseCertificate([]byte(certPEM))
}

// GenerateCSR 生成CSR
func GenerateCSR(domains []string) ([]byte, crypto.PrivateKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate private key failed: %w", err)
	}

	template := &x509.CertificateRequest{
		DNSNames: domains,
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create CSR failed: %w", err)
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	})

	return csrPEM, privateKey, nil
}

// PrivateKeyToPEM 私钥转PEM
func PrivateKeyToPEM(key crypto.PrivateKey) ([]byte, error) {
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		der, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, err
		}
		return pem.EncodeToMemory(&pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: der,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported key type")
	}
}

// SANDomainsToJSON SAN域名列表转JSON
func SANDomainsToJSON(domains []string) string {
	if len(domains) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(domains)
	return string(data)
}

// JSONToSANDomains JSON转SAN域名列表
func JSONToSANDomains(jsonStr string) []string {
	if jsonStr == "" || jsonStr == "[]" {
		return nil
	}
	var domains []string
	json.Unmarshal([]byte(jsonStr), &domains)
	return domains
}
