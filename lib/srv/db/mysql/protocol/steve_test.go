package protocol

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/siddontang/go-mysql/client"
	mysqllib "github.com/siddontang/go-mysql/mysql"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	cert, err := tls.LoadX509KeyPair(
		"/Users/stevehuang/.tsh/keys/teleport.dev.aws.stevexin.me/STeve-db/teleport.dev.aws.stevexin.me/local-x509.pem",
		"/Users/stevehuang/.tsh/keys/teleport.dev.aws.stevexin.me/STeve",
	)
	require.NoError(t, err)
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM([]byte(`-----BEGIN CERTIFICATE-----
MIIDzzCCAregAwIBAgIRAPg7QacI9Kwv1C/cpXCoYUgwDQYJKoZIhvcNAQELBQAw
gYAxJTAjBgNVBAoTHHRlbGVwb3J0LmRldi5hd3Muc3RldmV4aW4ubWUxJTAjBgNV
BAMTHHRlbGVwb3J0LmRldi5hd3Muc3RldmV4aW4ubWUxMDAuBgNVBAUTJzMyOTk1
NjIyMDA1OTgxMTczOTY1ODgyNzgyMTkwMDkzODg5NTY4ODAeFw0yMTExMjkyMDI3
NDVaFw0zMTExMjcyMDI3NDVaMIGAMSUwIwYDVQQKExx0ZWxlcG9ydC5kZXYuYXdz
LnN0ZXZleGluLm1lMSUwIwYDVQQDExx0ZWxlcG9ydC5kZXYuYXdzLnN0ZXZleGlu
Lm1lMTAwLgYDVQQFEyczMjk5NTYyMjAwNTk4MTE3Mzk2NTg4Mjc4MjE5MDA5Mzg4
OTU2ODgwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDAGdfxk4fPgq4Q
ba3e2o9ysMWcqFkxFAxXmXVSM5j8Gjrw6K8faum126wTs0DWOY8RMZRx03PaQL7M
bvAY6qeQ0kcuaas/b15yaPWw6jrf2KpX1lETKoVqz48vfqS1gDcm/6Gco+NcwnpH
1YikXaOs/QTnid0VpoEJxhgvc9a5Mz1EBIGbA9UQCPDEekSxtI8xyOPaNS4ys1cQ
bvHtn0gZVbqmGpVuo3/ApBmeND0xjeXMkDxzKiAzFPhro3hYkgt6p9lfTZSgrhoe
Fvzf0IvAMnf2Nh5v40d1X5/VX3Jc8N05GBjuxM+ipXToP5Btcv9cTtlmy8WiBe+F
K2sg7JrJAgMBAAGjQjBAMA4GA1UdDwEB/wQEAwIBpjAPBgNVHRMBAf8EBTADAQH/
MB0GA1UdDgQWBBQ+690qYGfyrYU1xcXXSq2qqfazyjANBgkqhkiG9w0BAQsFAAOC
AQEAadI4dtb8BwxcKwaxiANRX8W6EACI/U00QWRO6uDquYDvNvc2y7vJHvE6Kh+Y
z/GA6mkrDRSv0sb/Y8LArcByxaO+jkfS2Mkl8BP6ZK9u3yGwYJtxXc9pOZ7i0423
T4q+tvLAvkMlhZQIUvQ3PZmFqZPrjMlSx42LCP2MJlXihQHPjPMy3vTlhjRWarTC
K1YnUd+7FmNBhYGXsKeneO212y2LaohnSv/bJhNxtuIAvJ1gqhF4GLF8dCVYAodE
kihpNezhEdge6eQvso4ListDS5m5IodXtfPMCKajkoLSMDLvq/RGxdoBzs4PYI88
r49KlYx9PHPB3pL9E4WHkRRDEg==
-----END CERTIFICATE-----`))

	conn, err := client.Connect(
		"127.0.0.1:58581",
		"alice",
		"",
		"test",
		func(conn *client.Conn) {
			conn.SetTLSConfig(&tls.Config{
				RootCAs:            certPool,
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
			})
		},
	)

	require.NoError(t, err)

	result, err := conn.Execute("select * from user")
	require.NoError(t, err)
	defer result.Close()
	printRowDatas(result.RowDatas)

	stmt, err := conn.Prepare("select * from user where name = ? and age < ?")
	require.NoError(t, err)
	defer stmt.Close()

	result, err = stmt.Execute("steve", 200)
	require.NoError(t, err)
	printRowDatas(result.RowDatas)
}

func printRowDatas(data []mysqllib.RowData) {
	fmt.Println("--- steve row datas")
	for _, row := range data {
		fmt.Println(string(row))
	}
}
