# Auth Server Client


[Source file](https://github.com/gravitational/teleport/blob/master/auth/clt.go)
```go
type Client struct {
    roundtrip.Client
}
    Certificate authority endpoints control user and host CAs. They are
    central mechanism for authenticating users and hosts within the cluster.

    Client is HTTP API client that connects to the remote server

func NewClient(addr string, params ...roundtrip.ClientParam) (*Client, error)

func NewClientFromNetAddr(
    a utils.NetAddr, params ...roundtrip.ClientParam) (*Client, error)

func (c *Client) CheckPassword(user string, password []byte) error
    CheckPassword checks if the suplied web access password is valid.

func (c *Client) Delete(u string) (*roundtrip.Response, error)
    Delete issues http Delete Request to the server

func (c *Client) DeleteUser(user string) error
    DeleteUser deletes a user by username

func (c *Client) DeleteUserKey(username string, id string) error
    DeleteUserKey deletes a key by id for a given user

func (c *Client) DeleteWebSession(user string, sid string) error
    DeleteWebSession deletes a web session for this user by id

func (c *Client) DeleteWebTun(prefix string) error
    DeleteWebTun deletes the tunnel by prefix

func (c *Client) GenerateHostCert(
    key []byte, id, hostname string, ttl time.Duration) ([]byte, error)
    GenerateHostCert takes the public key in the Open SSH
    ``authorized_keys`` plain text format, signs it using Host CA private
    key and returns the resulting certificate.

func (c *Client) GenerateKeyPair(pass string) ([]byte, []byte, error)
    GenerateKeyPair generates SSH private/public key pair optionally
    protected by password. If the pass parameter is an empty string, the key
    pair is not password-protected.

func (c *Client) GenerateToken(fqdn string, ttl time.Duration) (string, error)
    GenerateToken creates a special provisioning token for the SSH server
    with the specified fqdn that is valid for ttl period seconds.

    This token is used by SSH server to authenticate with Auth server and
    get signed certificate and private key from the auth server.

    The token can be used only once and only to generate the fqdn specified
    in it.

func (c *Client) GenerateUserCert(
    key []byte, id, user string, ttl time.Duration) ([]byte, error)
    GenerateUserCert takes the public key in the Open SSH
    ``authorized_keys`` plain text format, signs it using User CA signing
    key and returns the resulting certificate.

func (c *Client) Get(u string, params url.Values) (*roundtrip.Response, error)
    Get issues http GET request to the server

func (c *Client) GetEvents() ([]interface{}, error)
    GetEvents returns last 20 audit events recorded by the auth server

func (c *Client) GetHostCAPub() ([]byte, error)
    Returns host certificate authority public key. This public key is used
    to validate if host certificates were signed by the proper key.

func (c *Client) GetLogWriter() *LogWriter
    GetLogWriter returns a io.Writer - compatible object that can be used by
    lunk.EventLogger to ship audit logs to the auth server

func (c *Client) GetServers() ([]backend.Server, error)
    GetServers returns the list of servers registered in the cluster.

func (c *Client) GetUserCAPub() ([]byte, error)
    Returns user certificate authority public key. This public key is used
    to check if the users certificate is valid and was signed by this
    authority.

func (c *Client) GetUserKeys(user string) ([]backend.AuthorizedKey, error)
    GetUserKeys returns a list of keys registered for this user. This list
    does not include the temporary keys associated with user web sessions.

func (c *Client) GetUsers() ([]string, error)
    GetUsers returns a list of usernames registered in the system

func (c *Client) GetWebSession(user string, sid string) (string, error)
    GetWebSession check if a web sesion is valid, returns session id in case
    if it is valid, or error otherwise.

func (c *Client) GetWebSessionsKeys(
    user string) ([]backend.AuthorizedKey, error)
    GetWebSessionKeys returns the list of temporary keys generated for this
    user web session. Each web session has a temporary user ssh key and
    certificate generated, that is stored for the duration of this web
    session. These keys are used to access SSH servers via web portal.

func (c *Client) GetWebTun(prefix string) (*backend.WebTun, error)
    GetWebTun retruns the web tunel details by it unique prefix

func (c *Client) GetWebTuns() ([]backend.WebTun, error)
    GetWebTuns returns a list of web tunnels supported by the system

func (c *Client) PostForm(
    endpoint string,
    vals url.Values,
    files ...roundtrip.File) (*roundtrip.Response, error)
    PostForm is a generic method that issues http POST request to the server

func (c *Client) ResetHostCA() error
    All host certificate keys will have to be regenerated and all SSH nodes
    will have to be re-provisioned after calling this method.

func (c *Client) ResetUserCA() error
    Regenerates user certificate authority private key. User authority
    certificate is used to sign User SSH public keys, so auth server can
    check if that is a valid key before even hitting the database.

    All user certificates will have to be regenerated.

func (c *Client) SignIn(user string, password []byte) (string, error)
    SignIn checks if the web access password is valid, and if it is valid
    returns a secure web session id.

func (c *Client) SubmitEvents(events [][]byte) error
    Submit events submits structured audit events in JSON serialized format
    to the auth server.

func (c *Client) UpsertPassword(user string, password []byte) error
    UpsertPassword updates web access password for the user

func (c *Client) UpsertServer(s backend.Server, ttl time.Duration) error
    UpsertServer is used by SSH servers to reprt their presense to the auth
    servers in form of hearbeat expiring after ttl period.

func (c *Client) UpsertUserKey(username string,
    key backend.AuthorizedKey, ttl time.Duration) ([]byte, error)
    UpsertUserKey takes public key of the user, generates certificate for it
    and adds it to the authorized keys database. It returns certificate
    signed by user CA in case of success, error otherwise. The certificate
    will be valid for the duration of the ttl passed in.

func (c *Client) UpsertWebTun(wt backend.WebTun, ttl time.Duration) error
    UpsertWebTun creates a persistent SSH tunnel to the specified web target
    server that is valid for ttl period. See backend.WebTun documentation
    for details


```
