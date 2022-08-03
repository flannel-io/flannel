package common

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"time"

	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

const (
	octetStream = "application/octet-stream"
)

type Client struct {
	region          string
	httpClient      *http.Client
	httpProfile     *profile.HttpProfile
	profile         *profile.ClientProfile
	credential      CredentialIface
	signMethod      string
	unsignedPayload bool
	debug           bool
	rb              *circuitBreaker
	logger          *log.Logger
}

func (c *Client) Send(request tchttp.Request, response tchttp.Response) (err error) {
	if request.GetScheme() == "" {
		request.SetScheme(c.httpProfile.Scheme)
	}

	if request.GetRootDomain() == "" {
		request.SetRootDomain(c.httpProfile.RootDomain)
	}

	if request.GetDomain() == "" {
		domain := c.httpProfile.Endpoint
		if domain == "" {
			domain = request.GetServiceDomain(request.GetService())
		}
		request.SetDomain(domain)
	}

	if request.GetHttpMethod() == "" {
		request.SetHttpMethod(c.httpProfile.ReqMethod)
	}

	tchttp.CompleteCommonParams(request, c.GetRegion())

	// reflect to inject client if field ClientToken exists and retry feature is enabled
	if c.profile.NetworkFailureMaxRetries > 0 || c.profile.RateLimitExceededMaxRetries > 0 {
		safeInjectClientToken(request)
	}

	if c.credential == nil {
		// Some APIs can skip signature.
		return c.sendWithoutSignature(request, response)
	} else if c.profile.DisableRegionBreaker == true || c.rb == nil {
		return c.sendWithSignature(request, response)
	} else {
		return c.sendWithRegionBreaker(request, response)
	}
}

func (c *Client) sendWithRegionBreaker(request tchttp.Request, response tchttp.Response) (err error) {
	defer func() {
		e := recover()
		if e != nil {
			msg := fmt.Sprintf("%s", e)
			err = tcerr.NewTencentCloudSDKError("ClientError.CircuitBreakerError", msg, "")
		}
	}()

	ge, err := c.rb.beforeRequest()

	if err == errOpenState {
		newEndpoint := request.GetService() + "." + c.rb.backupEndpoint
		request.SetDomain(newEndpoint)
	}
	err = c.sendWithSignature(request, response)
	isSuccess := false
	// Success is considered only when the server returns an effective response (have requestId and the code is not InternalError )
	if e, ok := err.(*tcerr.TencentCloudSDKError); ok {
		if e.GetRequestId() != "" && e.GetCode() != "InternalError" {
			isSuccess = true
		}
	}
	c.rb.afterRequest(ge, isSuccess)
	return err
}

func (c *Client) sendWithSignature(request tchttp.Request, response tchttp.Response) (err error) {
	if c.signMethod == "HmacSHA1" || c.signMethod == "HmacSHA256" {
		return c.sendWithSignatureV1(request, response)
	} else {
		return c.sendWithSignatureV3(request, response)
	}
}

func (c *Client) sendWithoutSignature(request tchttp.Request, response tchttp.Response) error {
	httpRequest, err := http.NewRequestWithContext(request.GetContext(), request.GetHttpMethod(), request.GetUrl(), request.GetBodyReader())
	if err != nil {
		return err
	}
	if request.GetHttpMethod() == "POST" {
		httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range request.GetHeader() {
		httpRequest.Header.Set(k, v)
	}
	httpResponse, err := c.sendWithRateLimitRetry(httpRequest, isRetryable(request))
	if err != nil {
		return err
	}
	err = tchttp.ParseFromHttpResponse(httpResponse, response)
	return err
}

func (c *Client) sendWithSignatureV1(request tchttp.Request, response tchttp.Response) (err error) {
	// TODO: not an elegant way, it should be done in common params, but finally it need to refactor
	request.GetParams()["Language"] = c.profile.Language
	err = tchttp.ConstructParams(request)
	if err != nil {
		return err
	}
	err = signRequest(request, c.credential, c.signMethod)
	if err != nil {
		return err
	}
	httpRequest, err := http.NewRequestWithContext(request.GetContext(), request.GetHttpMethod(), request.GetUrl(), request.GetBodyReader())
	if err != nil {
		return err
	}
	if request.GetHttpMethod() == "POST" {
		httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	for k, v := range request.GetHeader() {
		httpRequest.Header.Set(k, v)
	}

	httpResponse, err := c.sendWithRateLimitRetry(httpRequest, isRetryable(request))
	if err != nil {
		return err
	}
	err = tchttp.ParseFromHttpResponse(httpResponse, response)
	return err
}

func (c *Client) sendWithSignatureV3(request tchttp.Request, response tchttp.Response) (err error) {
	headers := map[string]string{
		"Host":               request.GetDomain(),
		"X-TC-Action":        request.GetAction(),
		"X-TC-Version":       request.GetVersion(),
		"X-TC-Timestamp":     request.GetParams()["Timestamp"],
		"X-TC-RequestClient": request.GetParams()["RequestClient"],
		"X-TC-Language":      c.profile.Language,
	}
	if c.region != "" {
		headers["X-TC-Region"] = c.region
	}
	if c.credential.GetToken() != "" {
		headers["X-TC-Token"] = c.credential.GetToken()
	}
	if request.GetHttpMethod() == "GET" {
		headers["Content-Type"] = "application/x-www-form-urlencoded"
	} else {
		headers["Content-Type"] = "application/json"
	}
	isOctetStream := false
	cr := &tchttp.CommonRequest{}
	ok := false
	var octetStreamBody []byte
	if cr, ok = request.(*tchttp.CommonRequest); ok {
		if cr.IsOctetStream() {
			isOctetStream = true
			// custom headers must contain Content-Type : application/octet-stream
			// todo:the custom header may overwrite headers
			for k, v := range cr.GetHeader() {
				headers[k] = v
			}
			octetStreamBody = cr.GetOctetStreamBody()
		}
	}

	for k, v := range request.GetHeader() {
		switch k {
		case "X-TC-Action", "X-TC-Version", "X-TC-Timestamp", "X-TC-RequestClient",
			"X-TC-Language", "Content-Type", "X-TC-Region", "X-TC-Token":
			c.logger.Printf("Skip header \"%s\": can not specify built-in header", k)
		default:
			headers[k] = v
		}
	}

	if !isOctetStream && request.GetContentType() == octetStream {
		isOctetStream = true
		b, _ := json.Marshal(request)
		var m map[string]string
		_ = json.Unmarshal(b, &m)
		for k, v := range m {
			key := "X-" + strings.ToUpper(request.GetService()) + "-" + k
			headers[key] = v
		}

		headers["Content-Type"] = octetStream
		octetStreamBody = request.GetBody()
	}
	// start signature v3 process

	// build canonical request string
	httpRequestMethod := request.GetHttpMethod()
	canonicalURI := "/"
	canonicalQueryString := ""
	if httpRequestMethod == "GET" {
		err = tchttp.ConstructParams(request)
		if err != nil {
			return err
		}
		params := make(map[string]string)
		for key, value := range request.GetParams() {
			params[key] = value
		}
		delete(params, "Action")
		delete(params, "Version")
		delete(params, "Nonce")
		delete(params, "Region")
		delete(params, "RequestClient")
		delete(params, "Timestamp")
		canonicalQueryString = tchttp.GetUrlQueriesEncoded(params)
	}
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\n", headers["Content-Type"], headers["Host"])
	signedHeaders := "content-type;host"
	requestPayload := ""
	if httpRequestMethod == "POST" {
		if isOctetStream {
			// todo Conversion comparison between string and []byte affects performance much
			requestPayload = string(octetStreamBody)
		} else {
			b, err := json.Marshal(request)
			if err != nil {
				return err
			}
			requestPayload = string(b)
		}
	}
	hashedRequestPayload := ""
	if c.unsignedPayload {
		hashedRequestPayload = sha256hex("UNSIGNED-PAYLOAD")
		headers["X-TC-Content-SHA256"] = "UNSIGNED-PAYLOAD"
	} else {
		hashedRequestPayload = sha256hex(requestPayload)
	}
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedRequestPayload)
	//log.Println("canonicalRequest:", canonicalRequest)

	// build string to sign
	algorithm := "TC3-HMAC-SHA256"
	requestTimestamp := headers["X-TC-Timestamp"]
	timestamp, _ := strconv.ParseInt(requestTimestamp, 10, 64)
	t := time.Unix(timestamp, 0).UTC()
	// must be the format 2006-01-02, ref to package time for more info
	date := t.Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, request.GetService())
	hashedCanonicalRequest := sha256hex(canonicalRequest)
	string2sign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		requestTimestamp,
		credentialScope,
		hashedCanonicalRequest)
	//log.Println("string2sign", string2sign)

	// sign string
	secretDate := hmacsha256(date, "TC3"+c.credential.GetSecretKey())
	secretService := hmacsha256(request.GetService(), secretDate)
	secretKey := hmacsha256("tc3_request", secretService)
	signature := hex.EncodeToString([]byte(hmacsha256(string2sign, secretKey)))
	//log.Println("signature", signature)

	// build authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		c.credential.GetSecretId(),
		credentialScope,
		signedHeaders,
		signature)
	//log.Println("authorization", authorization)

	headers["Authorization"] = authorization
	url := request.GetScheme() + "://" + request.GetDomain() + request.GetPath()
	if canonicalQueryString != "" {
		url = url + "?" + canonicalQueryString
	}
	httpRequest, err := http.NewRequestWithContext(request.GetContext(), httpRequestMethod, url, strings.NewReader(requestPayload))
	if err != nil {
		return err
	}
	for k, v := range headers {
		httpRequest.Header[k] = []string{v}
	}
	httpResponse, err := c.sendWithRateLimitRetry(httpRequest, isRetryable(request))
	if err != nil {
		return err
	}
	err = tchttp.ParseFromHttpResponse(httpResponse, response)
	return err
}

// send http request
func (c *Client) sendHttp(request *http.Request) (response *http.Response, err error) {
	if c.debug {
		outBytes, err := httputil.DumpRequest(request, true)
		if err != nil {
			c.logger.Printf("[ERROR] dump request failed because %s", err)
			return nil, err
		}
		c.logger.Printf("[DEBUG] http request = %s", outBytes)
	}

	response, err = c.httpClient.Do(request)
	return response, err
}

func (c *Client) GetRegion() string {
	return c.region
}

func (c *Client) Init(region string) *Client {
	c.httpClient = &http.Client{}
	c.region = region
	c.signMethod = "TC3-HMAC-SHA256"
	c.debug = false
	c.logger = log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)
	return c
}

func (c *Client) WithSecretId(secretId, secretKey string) *Client {
	c.credential = NewCredential(secretId, secretKey)
	return c
}

func (c *Client) WithCredential(cred CredentialIface) *Client {
	c.credential = cred
	return c
}

func (c *Client) GetCredential() CredentialIface {
	return c.credential
}

func (c *Client) WithProfile(clientProfile *profile.ClientProfile) *Client {
	c.profile = clientProfile
	if c.profile.DisableRegionBreaker == false {
		c.withRegionBreaker()
	}
	c.signMethod = clientProfile.SignMethod
	c.unsignedPayload = clientProfile.UnsignedPayload
	c.httpProfile = clientProfile.HttpProfile
	c.debug = clientProfile.Debug
	c.httpClient.Timeout = time.Duration(c.httpProfile.ReqTimeout) * time.Second
	return c
}

func (c *Client) WithSignatureMethod(method string) *Client {
	c.signMethod = method
	return c
}

func (c *Client) WithHttpTransport(transport http.RoundTripper) *Client {
	c.httpClient.Transport = transport
	return c
}

func (c *Client) WithDebug(flag bool) *Client {
	c.debug = flag
	return c
}

// WithProvider use specify provider to get a credential and use it to build a client
func (c *Client) WithProvider(provider Provider) (*Client, error) {
	cred, err := provider.GetCredential()
	if err != nil {
		return nil, err
	}
	return c.WithCredential(cred), nil
}

func (c *Client) withRegionBreaker() *Client {
	rb := defaultRegionBreaker()
	if c.profile.BackupEndpoint != "" {
		rb.backupEndpoint = c.profile.BackupEndpoint
	} else if c.profile.BackupEndPoint != "" {
		rb.backupEndpoint = c.profile.BackupEndPoint
	}
	c.rb = rb
	return c
}

func NewClientWithSecretId(secretId, secretKey, region string) (client *Client, err error) {
	client = &Client{}
	client.Init(region).WithSecretId(secretId, secretKey)
	return
}

// NewClientWithProviders build client with your custom providers;
// If you don't specify the providers, it will use the DefaultProviderChain to find credential
func NewClientWithProviders(region string, providers ...Provider) (client *Client, err error) {
	client = (&Client{}).Init(region)
	var pc Provider
	if len(providers) == 0 {
		pc = DefaultProviderChain()
	} else {
		pc = NewProviderChain(providers)
	}
	return client.WithProvider(pc)
}
