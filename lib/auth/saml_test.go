/*
Copyright 2019-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/xml"
	"net/url"
	"testing"
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/jonboulle/clockwork"
	saml2 "github.com/russellhaering/gosaml2"
	samltypes "github.com/russellhaering/gosaml2/types"
	"github.com/stretchr/testify/require"
)

func TestCreateSAMLUser(t *testing.T) {
	c := clockwork.NewFakeClockAt(time.Now())
	b, err := lite.NewWithConfig(context.Background(), lite.Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            c,
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                b,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}

	a, err := NewServer(authConfig)
	require.NoError(t, err)

	// Dry-run creation of SAML user.
	user, err := a.createSAMLUser(&createUserParams{
		connectorName: "samlService",
		username:      "foo@example.com",
		roles:         []string{"admin"},
		sessionTTL:    1 * time.Minute,
	}, true)
	require.NoError(t, err)
	require.Equal(t, "foo@example.com", user.GetName())

	// Dry-run must not create a user.
	_, err = a.GetUser("foo@example.com", false)
	require.Error(t, err)

	// Create SAML user with 1 minute expiry.
	_, err = a.createSAMLUser(&createUserParams{
		connectorName: "samlService",
		username:      "foo@example.com",
		roles:         []string{"admin"},
		sessionTTL:    1 * time.Minute,
	}, false)
	require.NoError(t, err)

	// Within that 1 minute period the user should still exist.
	_, err = a.GetUser("foo@example.com", false)
	require.NoError(t, err)

	// Advance time 2 minutes, the user should be gone.
	c.Advance(2 * time.Minute)
	_, err = a.GetUser("foo@example.com", false)
	require.Error(t, err)
}

func TestEncryptedSAML(t *testing.T) {
	// This Base64 encoded XML blob is a signed SAML response with an encrypted assertion for testing decryption and parsing.
	const EncryptedResponse = `PD94bWwgdmVyc2lvbj0iMS4wIj8+DQo8c2FtbHA6UmVzcG9uc2UgeG1sbnM6c2FtbHA9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpwcm90b2NvbCIgeG1sbnM6c2FtbD0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOmFzc2VydGlvbiIgSUQ9InBmeDBmNTBiYTg0LWVmNjctNTQyZi1kZDgyLTI4NTU0MzVlMGM4MCIgVmVyc2lvbj0iMi4wIiBJc3N1ZUluc3RhbnQ9IjIwMTQtMDctMTdUMDE6MDE6NDhaIiBEZXN0aW5hdGlvbj0iaHR0cDovL3NwLmV4YW1wbGUuY29tL2RlbW8xL2luZGV4LnBocD9hY3MiIEluUmVzcG9uc2VUbz0iT05FTE9HSU5fNGZlZTNiMDQ2Mzk1YzRlNzUxMDExZTk3Zjg5MDBiNTI3M2Q1NjY4NSI+DQogIDxzYW1sOklzc3Vlcj5odHRwOi8vaWRwLmV4YW1wbGUuY29tL21ldGFkYXRhLnBocDwvc2FtbDpJc3N1ZXI+PGRzOlNpZ25hdHVyZSB4bWxuczpkcz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+DQogIDxkczpTaWduZWRJbmZvPjxkczpDYW5vbmljYWxpemF0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8xMC94bWwtZXhjLWMxNG4jIi8+DQogICAgPGRzOlNpZ25hdHVyZU1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNyc2Etc2hhMSIvPg0KICA8ZHM6UmVmZXJlbmNlIFVSST0iI3BmeDBmNTBiYTg0LWVmNjctNTQyZi1kZDgyLTI4NTU0MzVlMGM4MCI+PGRzOlRyYW5zZm9ybXM+PGRzOlRyYW5zZm9ybSBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNlbnZlbG9wZWQtc2lnbmF0dXJlIi8+PGRzOlRyYW5zZm9ybSBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMTAveG1sLWV4Yy1jMTRuIyIvPjwvZHM6VHJhbnNmb3Jtcz48ZHM6RGlnZXN0TWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnI3NoYTEiLz48ZHM6RGlnZXN0VmFsdWU+TUxic3U4WFFOcW4xWE8walUzeHZIL0pPalZnPTwvZHM6RGlnZXN0VmFsdWU+PC9kczpSZWZlcmVuY2U+PC9kczpTaWduZWRJbmZvPjxkczpTaWduYXR1cmVWYWx1ZT5yVTVDUzhWQnZGVjl3RkUvOEY1NHROQTd3UVFWbG9UZkRsL0h1amJwRzJBWTNZcExtdWxzU2pOdngvc0F4a3luZ0lLTVE2dHphZkN3KzZjaGNldzh4bUNOcWdSNWNiQ09DbzB2UUJXaXhINm9jU2FKWDRTU21WeEhhU2p1clRNRkZnamFFYktiM2duV21haGpDb093TU9MZHJtWlprYkp2OWQrWTVUR0VYL2hhUmMvbXU2b05WT3dCL0xMdURDdzk3RTkxdVNUVUpvL1RPS0tVRjJYenZhVEEwMXZobzM5OTYvalpFWkRYR1ZyTGlkOTg5NDJXWWVjT3F6ZnZTWWtLemNaRGd2ZE1udlR1M20yVHpXQ1RqaEVzVDN1cjQ2OThIUmlyZUZTbnFINldhYUVMYjFFeGFiemdxNGFsRG9ma1J3ZU14YWJleVV6aUVBSWhKOGYrNVE9PTwvZHM6U2lnbmF0dXJlVmFsdWU+DQo8ZHM6S2V5SW5mbz48ZHM6WDUwOURhdGE+PGRzOlg1MDlDZXJ0aWZpY2F0ZT5NSUlES2pDQ0FoS2dBd0lCQWdJUUp0SkRKWlpCa2cvYWZNOGQyWkpDVGpBTkJna3Foa2lHOXcwQkFRc0ZBREJBTVJVd0V3WURWUVFLRXd4VVpXeGxjRzl5ZENCUFUxTXhKekFsQmdOVkJBTVRIblJsYkdWd2IzSjBMbXh2WTJGc2FHOXpkQzVzYjJOaGJHUnZiV0ZwYmpBZUZ3MHhOekExTURreE9UUXdNelphRncweU56QTFNRGN4T1RRd016WmFNRUF4RlRBVEJnTlZCQW9UREZSbGJHVndiM0owSUU5VFV6RW5NQ1VHQTFVRUF4TWVkR1ZzWlhCdmNuUXViRzlqWVd4b2IzTjBMbXh2WTJGc1pHOXRZV2x1TUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUF1S0ZMYWYyaUlJL3hEUittMllqNlBuVUVhK3F6cXd4c2RMVWpudW5GWmFBWEcraFptNE1sODBTQ2lCZ0lnVEhRbEp5TElrVHR1Um9INWFlTXl6MUVSVUN0aWk0WnNUcURyampVeWJ4UDRyKzRIVlg2bTM0czZod0VyOEZpZnRzOXBNcDRpUzN0UWd1UmMyOGdQZERvL1Q2VnJKVFZZVWZVVXNORFJ0SXJsQjVPOWlncXFMbnVhWTllcUdpNFBVeDBHMHdSWUpwUnl3b2o4RzBJa3BmUVRpWCtDQUM3ZHQ1d3M3WnJuR3FDTkJMR2k1YkdzYU1tcHRWYnNTRXAxVGVubnRGNTRWMWlSNDlJVjVKcURobTFTMEhta2xlb0p6S2RjKzZzUC94TmVwejlQSnp1RjlkOU51YlRMV2dCc0syOFlJdGNtV0hkSFhEL09EeFZhZWhSandJREFRQUJveUF3SGpBT0JnTlZIUThCQWY4RUJBTUNCNEF3REFZRFZSMFRBUUgvQkFJd0FEQU5CZ2txaGtpRzl3MEJBUXNGQUFPQ0FRRUFBVlU2c05CZGo3NnNhSHdPeEdTZG5FcVFvMnRNdVIzbXNTTTRGNndGSzJVa0tlcHNEN0NZSWYvUHpOU05VcUE1SklFVVZlTXFHeWlIdUFiVTRDNjU1blQxSXlKWDFELytyNzNzU3A1amJJcFFtMnhvUUdabmo2Zy9LbHR3OE9TT0F3K0RzTUYvUExWcW9XSnAwN3U2ZXcvbU54V3NKS2NaNWsrcTRlTXhjaTltS1JISHFzcXVXS1h6UWxVUk1ORkkrbUdhRndyS000ZG16YVIwQkVjK2lsU3hRcVV2UTc0c21zTEsremhOaWttZ2psR0M1b2I5ZzhYa2hWQWtKTUFoMnJiOW9uRE5pUmw2OGlBZ2N6UDg4bVh1dk4vbzk4ZHlwenNQeFhtdzZ0a0RxSVJQVUFVYmg0NjVybFk1c0tNbVJnWGkyclVmbC9RVjVuYm96VW8vSFE9PTwvZHM6WDUwOUNlcnRpZmljYXRlPjwvZHM6WDUwOURhdGE+PC9kczpLZXlJbmZvPjwvZHM6U2lnbmF0dXJlPg0KICA8c2FtbHA6U3RhdHVzPg0KICAgIDxzYW1scDpTdGF0dXNDb2RlIFZhbHVlPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6c3RhdHVzOlN1Y2Nlc3MiLz4NCiAgPC9zYW1scDpTdGF0dXM+DQogIA0KPHNhbWw6RW5jcnlwdGVkQXNzZXJ0aW9uPjx4ZW5jOkVuY3J5cHRlZERhdGEgeG1sbnM6eGVuYz0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjIiB4bWxuczpkc2lnPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIiBUeXBlPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNFbGVtZW50Ij48eGVuYzpFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyIvPjxkc2lnOktleUluZm8geG1sbnM6ZHNpZz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+PHhlbmM6RW5jcnlwdGVkS2V5Pjx4ZW5jOkVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNyc2Etb2FlcC1tZ2YxcCIvPjx4ZW5jOkNpcGhlckRhdGE+PHhlbmM6Q2lwaGVyVmFsdWU+TjlLaHFKeWtJdGk1eVZETzdzT0VpT2lMb1VWL2p5aEdITU0wZmFTUzZnWVJ5RUZqaFR2RzNCUEpsd1RTTXpzTTFuY1pwTGVBd0FKaFVzci9mT0pCVGtQbjA3UzZqZGsxYTBMaU9EbjkrcDlCVXlidjRXYWsyWGduMnhXNytDNVQ5bGhvQ0dHRThrdHh6Q0tXL1FhWUNWV3RsMEp1TGdNYWIyWHUzL1dJdVlSWDhKbVZ2ckdPWTlOd0hpeFhFT09PbDFSUUNnbXpNaCtxUno5eFhwWGU0Sk1XRGNqQ0g2blk5V2Fxem4yQTNJVnJzS1V3bUZuTWFaM1lOM04ybmNROWo2QXRYSThoWEErMjBvRlhBQWx2c3JVK0xOek9hTFRzb1QySFZVUER0YldhQm9tS1cxdW9lc1hPZG1KNnVDUVc4Zk9BL3p0QStjL1JERTdyaTZiSFNERCs3YW5uQlNaRzVzL1lrdm5PT2wxRjRFYndLdVpMd3RWdjQza1B1dnRVeUVxTE1HRFlhNmtXczlLRjRsR0dqa0YxMXpqTmVnSTRFd09EVXhZSm14QldRNXhvVm1sOGVvK0VNdGt5NzFGeUttMFhpSGo0cWw5Qnl4dUtaVHJrTXdjQjlPSkNFKzcwM1dpdW5uZy9OSGkrV1IrTmhORlZqUm40SWQ3UTVFTEZZNFRNMklNQld6Y3R2cEd6TGVHdjF2L2RXanVodU1aVjJpeG5nMzRxNXg3VVVnODAwNVlRTnNwOVcwYVZKdDRnY0tUeFAyRnJTVGVjM0MxRktVT1JXSCtVc2ZpZ21GY09YdHFvQXgyK3dDZGs3MkVTS3Z2WFYvWEVFSmd0aTRiVldUK0dzSmc2eGRhTjBPNnlpWTg4eE5BcWZMUEcwajJTb0k1bUcxYzM0YVE9PC94ZW5jOkNpcGhlclZhbHVlPjwveGVuYzpDaXBoZXJEYXRhPjwveGVuYzpFbmNyeXB0ZWRLZXk+PC9kc2lnOktleUluZm8+DQogICA8eGVuYzpDaXBoZXJEYXRhPg0KICAgICAgPHhlbmM6Q2lwaGVyVmFsdWU+a1dWbkpzTkZZZ1JTNlMvTlFERVlsN3RTTjVsTzR6YWtqdXdxTDROMlFHbW9rdnBWRlJreFp3bjVhOXkrVDhLUGZpOWt2bEhlNktzL1I1UTJ2NkdVd0Z4V1dPYVZrOFJDMUh1blN1ZnZ5Tks1Y21hRmJSM0t3OFZZZnNPRVV2T0d6Y2pqTFFjSFFFUEZUMXJsMW0vazc3dExzbFlxV1B6WkRhUFpkU1lkd0kvdlo0OThIeXF3b0Y4U2tCcFVQcW02bnYwVGFVdU5pOWMzbkdDUTBWejE3UGFnLzN3SERWTnA3RlNsSFJlSStySkZsS0RXalF4MStDZjF6U3pjTG5Ecll1aEt3MWY3WVVTYUh0enlBL2l5MkhRMDdSZjRyNUpRaUorR2FOSUkwYk44RTZ0QXJqbjFZSGZWMDRQdWNwa0xDTEJTRndiOERXZE9wMnFEN3pvcEJvc2g2YVF6Vjh3QjhsUEFqUUd2aFhlQVdwRmsxWkVaUG5SRkkydkdkaGFHL2o5TkQvM0EvbzlvZFZpd1ZBdFd4SG9WVzZWcWtLOG5GQlVyL25IVDZTYmdsOFlUd2s4N0hRaldHUEdSaksyNEp3YXc0VjV6djN4bG9zSlFpdnc2dGwyVXk5YWROdUlyOUFCRGJuc3RmdXlQNEZaenR2Q0RTUHFPZlVVQlhya0thczNJRE1iNm9KREQ3a003QVdHcEU5K0lJT0NoVzdiRk5hb3RndHN1OTVqd1ZNQ3BVL0NkNkhPbURrMWpDYVc5RzhlQWJVdjhaUEVmN1Q5c0Z1VnZOaVBVbkFyMWV1VEl6WFNPekFtZk5jdXZpZ1BnOUlCYzAvT0pYWjBBVTgvQWxyRTZRSldwL2pDYThUNTF0YTFjbVN6SGQ4SG0zTEt5aDhsYjljUG5RR3RCeW9LcFUzZEMwQWk3OU1Lc1NhekRTalByY2l6ZUdhS0Vzd1NCTWtWRGtDWFMybGJicXBrckxvN2tMdy9TNG1OMWZCVjU5a2txd1ZlL3pKQXJNNllIckNJUURwRkRCSWtHeXE1WVl1VkQyeXk4YnJmeFNBemMyK2ZpeWQ5OVJndEtTeGZhVkROV3VJZzJnVTRQME40TnVTOCtrb0MzclF6eWovOGdWSlRpSGg5UTFEOVRJbjhjZGllTE12ZlZLT25oMHZBTnR0MDN2YnRHVkJxTTQwa3VLeUxKRWE5TWY0N2xvWk5qamUxd2VvWW1wblZScGVxVGxzUDRzVmwzd3QwYWlDZG5mdHVaVlVWb0M3Um51VDRidVRWZVgybUdNMElxR242czJwZ0xsSU5xVEFTY0F0K3FlVEhycmVTcUhFSmovZnhyM3NySEpyMHZjQ0w2enZZMWtOUi9taGcyL25Bc0hwTkc3c21ITS9oQ3gzOS9HZ01FeFpXbU5lVXV3amhhOHpWc3FRdnZ6cDM3Nk1OWC9xeERuU0JxTEorTERlRXg0SnYweGJxd0tvVE01L3BSQThDOVA4NWRqZXYwT1RKWVBEOGZzNHhabmV6NHRQTlZhcEc1RzEwWG1Wem9TYm1MNXdYamV0bXJJY0NMZ0laeEZnZ1NFSmsxUGFtdThzU2VraHBNOE1EbVZXejZrRDZxdUxyRXNqc0h2K0Rxc1REMkJIRjJMOVVuMjlaL0NvbFBNaDRuT0NRZHpjcjNUT0dEalJaMk1lanJHb2VnblVORW5MaGhwK2luWTB5L2lsODZYV2FweERoUEFLclIxS3pTRUJ5Yk5UUE81S0o4M0pBMGx0dE5JVk9YZ3FkZkQxSy9KYnM4ZHdYY3pRenZCRXBMOXlEZVN4d0wzVXR2VG80NEJLVkYxUVdwUDVvdHU5ZndpZXNkZlJLZm1zM3pteFZiQTZ4dkprK0M2MkE2YXBDcjhqL3FaUldNTUczNytnMUI4bU13eGdabkxyOE9ic2JMQjBVV3JOYWdMbldpc29ZQTVmclcxN21wbW9tc3V1cmhQQ2IrbGczbmFkK1BCRXpXaUZISnJvVkdQMFRwZ1NWZlZvSU9wYkZCS2JpWjVoUnd5YURuaFMrL1UreXNUZ1hXdE1qUk9QYjRROVZiUW8vSXd0NEpVZjZNUlVlN0FoaTNUbTUyOUR5Q1ppOFNydTBtRDF4ZHArdjlkSXF2ZEoxZXROYXZFMTlCSmRDSzN0VHhIZHk2ODh6bEo4NFJOUTlxYzI1R3pudXVFNFg3ZjBkUkQrdEoyNkJNVmh5L3RLeDRzdUpIVURzS05yUmZ4aGM4czNwU2FhUTFZS0E4eVpnQnBIN2tDY0FJZ2g5NlpEU3NuUngzdU5Zc0txakFmd0x3TkNQam02bHY1YVV1azBtYklMelBwMDhFRWFzRWNhWE1CcEZraERKTGhIS1l2WUhPdjlQYTZvWEFOVzZSOC9rSUxoT2ppbnl3RmZUZ2FDci96R20zcitjc2VoM0RxTGNjZjRteXY3Tmp5eERUSWtVeUVDdDJLYnI3S2ZPckh3T3I0L3M0ODVhVVVJVDRxYml5Lzd4U0E1NXlDaFUzV0RkaGpDQ3l2WXNSa3cvUjhVQUMyRExmZWxDVlducWdoTnE3blhNdU9zM3h4L3hpLzE5eEVYK3RxTWNqUHVsc3FQejk4WDF1UE5iZDluYXpSUnFEaXQ2R0Y2NHNlY1ZKVmVjZlpHNTJpTHpRbzZLNzVtdnU0M00rNlI5MkhDdkVWd09sRDhCRWoxYncrUWFaL3FDNVF4UjNtMkR5VHlmd20wVVVUM25yOG0yTDdaR3ozTUU2RXViTzZFWFBVR0JpRnlUeTNNUzRxVGUrbTJoWHdwd0xhTXM3SXFjRVBvMW5KeUJmN1plWm1kZVpsLzJHZ1pVNk94YmxiTmtUZUJORlRWVjhzK0Y2K2h3ZVArMzdBbEVsVWNGNTZaUk9NU0Nmemw0OFRtR1BZNHdpRmlMWGErMXZqTFN6NGdlYzhlVGZJcXczdEdaNjAvUTJGRGV6TUV2MEd0cSsxbTU3VXpqWGtLSVoyNzJBZ3VUNzZsbWUyd1B3dUVtOER2ZHE0Z21ZZlE5MVZ5M3J3aElsVVpvakRxU2R0UHJiTU4zK0JvR1Y0SVFTdUExNlN0UUtJWkg3VVNkckZBZ1dhLzVjQzUwVjIvMUVMMWZRWjJFa01LbktFczRnc2hZNU5YRTYzTmJTSjZiMnpKQll6MDdRcXVTemhycGxUeTR3dVpqcytRd082bUhDZkdEaFY4R2I1RG1HcnExMVVJNVllNVp4RXNLS0FvSlRYdWFOZlRpamFKb2l5L2lvcmFZZk00eUNHUVIyU04wdnc4Zk9LdXA3SlIrejk5TXdmSEJwV21LaHhFM3dITWpZOWpJa3Nyc3JLWnk3MnFRaGROMDZCdEo2SFdpN25PNWJUdHVQZlpmUFdPOFVEZUpKZFdLdUVUNUdpeWFxWGhOM0VwQUQvcDA5RjZkOEgrb1gyRVAzQjRZWXE2cTlSOHpJZG9kZ3cxbncxZjM3MERpL0pSSG1PbnQ5VEord05FS0x5eW42dFVodXRpOGZMY2VKYkk2dHJFb1I2S3NZWGJUeXprVXg5TGtiVkNFelVicVV1YmtrdG5WVm5rcHFJSmNFTGdpZWl0TFRTYW80ZkdOaFROVDVCWDEySWtoQWsrSEhSZlZtWjdhY0lBNDlLYkhlRElpUGxOR3A1WGxMQ1luZGxlOFZlWDF4bEw4Y0duZlB5QWtiSTRLN3RQR2h4S0YzSWFoazFtTVFGWFRhQ3FvdS9pYm9ZQ0F0R3pod3pOaUtLcXhRT2FSWFpQWkY3TlVKbE13NlR1VFhhaU9pM0dLMDZ4eXY2ZEcxTk9ya2daRmhNSS9CZHdjWVcxaHdCaElXWHBVTnpNUyt6cDVTS1BOQW5BSHE4aW16OU1JcGNQajFkYXo0cUNQd3N4dXMwYXBnMkdZQ0xYTW8zdnlTRXd3WTAwQjZzUUlrRHhLTUc4TWtvQjFpMU5vVGMwNnJxTkFOL2szZ3h4UnpBNUFpNHU4NGhpd3dxOXlMWC9oNjFGRnpySklEY3gydHl4MHdUTFd2SzNBWCtOZStQMDZvWWEyVHN4R0RkV0RQR1BFc2U0aFZlTUl6PC94ZW5jOkNpcGhlclZhbHVlPg0KICAgPC94ZW5jOkNpcGhlckRhdGE+DQo8L3hlbmM6RW5jcnlwdGVkRGF0YT48L3NhbWw6RW5jcnlwdGVkQXNzZXJ0aW9uPjwvc2FtbHA6UmVzcG9uc2U+`

	// This XML blob is a sample EntityDescriptor made to satisfy the connector validator for testing.
	const EntityDescriptor = `<?xml version="1.0"?>
	<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2021-02-26T15:57:24Z" cacheDuration="PT1614787044S" entityID="http://some.entity.id">
	  <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
		<md:KeyDescriptor use="signing">
		  <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
			<ds:X509Data>
			  <ds:X509Certificate>MIIFazCCA1OgAwIBAgIUDpXWZ8npv3sWeCQbB1WCwMoDe9QwDQYJKoZIhvcNAQELBQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMTAyMTgyMTUyNTVaFw0yMjAyMTgyMTUyNTVaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDiEvFfAwgR8rfFPXVkJiWQGisFQNpQ5oq4ng5sD/3phPBBzwx0TTn+V+XG5pBTlyVe0h9kLqZ3Dnavdk9VDC1DIrc0CSKUhP01JdV9TlC/tCek9a2IQEjEZ0pZPbU/gtXxEGyrs9JVFf0K8saMH6xB8jJwB4Eq9jB8rsWZJh4HeyX1VEdruPdwRkFjuNhBnIax//DQSZepAhtM+mtxP+cHtRzXPlXHTpYvxcP2LoXjSdCh/XEu8Ai33O4Ek14HIFmNQ63pmzmxhpcPm8ejDFchOEU67zeOz2RQNAefeHRgG1gvFIcgmVXcLM+VmC0JlzNuyMFY1XUygm1PYcFz93p4OGJBkYgKifNHPcMzTLQtPoY397WREd/kkMtvgxSDs6GQr2VwByHoo5IoQJ/OpridaDduL9NSc6YHEEXxSceMSdI+txuZvOAJJuLR1DQ5S5xjdHBj8uDsAnmX7oORVadEJ38Aj1UlM+Lk6qnmoBEGAXEfa3Fxyz0qgN9MrtutJO0S4BLqqmXgM9Kulp0B7e7gkRaAyNt/Y0+dAuzYva+uTd7Qm96EEYCTwd9LM4OghTLpDCXFm5EQI+D0zEyOGhDqwQDdx3MHJoPd6xg72ZkoiADY235D/av/ZisF7acPucLvQ41gbWphQgsRTN81lRll/Wgd4EknznXq060RQBkNbwIDAQABo1MwUTAdBgNVHQ4EFgQUzpwOh72T7DyvsvkVV9Cu4YRKBTYwHwYDVR0jBBgwFoAUzpwOh72T7DyvsvkVV9Cu4YRKBTYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAgEADSc0AEFgMcwArn9zvppOdMlF4GqyJa7mzeVAKHRyXiLm4TSUk8oBk8GgO9f32B5sEUVBnL5FnzEUm7hMAG5DUcMXANkHguIwoISpAZdFh1VhH+13HIOmxre/UN9a1l829g1dANvYWcoGJc4uUtj3HF5UKcfEmrUwISimW0Mpuin+jDlRiLvpvImqxWUyFazucpE8Kj4jqmFNnoOLAQbEerR61W1wC3fpifM9cW5mKLsSpk9uG5PUTWKA1W7u+8AgLxvfdbFA9HnDc93JKWeWyBLX6GSeVL6y9pOY9MRBHqnpPVEPcjbZ3ZpX1EPWbniF+WRCIpjcye0obTTjipWJli5HqwGGauyXPGmevCkG96jiy8nf18HrQ3459SuRSZ1lQD5EoF+1QBL/O1Y6P7PVuOSQev376RD56tOLu1EWxZAmfDNNmlZSmZSn+h5JRcjSh1NFfktIVkHtNPKw8FXDp8098oqrJ3MoNTQgE0vpXiho1QIxWhfaEU5y/WynZFk1PssjBULWNxbeIpOFYk3paNyEpb9cOkOE8ZHOdi7WWJSwHaDmx6qizOQXO75QMLIMxkCdENFx6wWbNMvKCxOlPfgkNcBaAsybM+K0AHwwvyzlcpVfEdaCexGtecBoGkjFRCG+f9InppaaSzmgbIJvkSOMUWEDO/JlFizzWAG8koM=</ds:X509Certificate>
			</ds:X509Data>
		  </ds:KeyInfo>
		</md:KeyDescriptor>
		<md:KeyDescriptor use="encryption">
		  <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
			<ds:X509Data>
			  <ds:X509Certificate>MIIFazCCA1OgAwIBAgIUDpXWZ8npv3sWeCQbB1WCwMoDe9QwDQYJKoZIhvcNAQELBQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMTAyMTgyMTUyNTVaFw0yMjAyMTgyMTUyNTVaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDiEvFfAwgR8rfFPXVkJiWQGisFQNpQ5oq4ng5sD/3phPBBzwx0TTn+V+XG5pBTlyVe0h9kLqZ3Dnavdk9VDC1DIrc0CSKUhP01JdV9TlC/tCek9a2IQEjEZ0pZPbU/gtXxEGyrs9JVFf0K8saMH6xB8jJwB4Eq9jB8rsWZJh4HeyX1VEdruPdwRkFjuNhBnIax//DQSZepAhtM+mtxP+cHtRzXPlXHTpYvxcP2LoXjSdCh/XEu8Ai33O4Ek14HIFmNQ63pmzmxhpcPm8ejDFchOEU67zeOz2RQNAefeHRgG1gvFIcgmVXcLM+VmC0JlzNuyMFY1XUygm1PYcFz93p4OGJBkYgKifNHPcMzTLQtPoY397WREd/kkMtvgxSDs6GQr2VwByHoo5IoQJ/OpridaDduL9NSc6YHEEXxSceMSdI+txuZvOAJJuLR1DQ5S5xjdHBj8uDsAnmX7oORVadEJ38Aj1UlM+Lk6qnmoBEGAXEfa3Fxyz0qgN9MrtutJO0S4BLqqmXgM9Kulp0B7e7gkRaAyNt/Y0+dAuzYva+uTd7Qm96EEYCTwd9LM4OghTLpDCXFm5EQI+D0zEyOGhDqwQDdx3MHJoPd6xg72ZkoiADY235D/av/ZisF7acPucLvQ41gbWphQgsRTN81lRll/Wgd4EknznXq060RQBkNbwIDAQABo1MwUTAdBgNVHQ4EFgQUzpwOh72T7DyvsvkVV9Cu4YRKBTYwHwYDVR0jBBgwFoAUzpwOh72T7DyvsvkVV9Cu4YRKBTYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAgEADSc0AEFgMcwArn9zvppOdMlF4GqyJa7mzeVAKHRyXiLm4TSUk8oBk8GgO9f32B5sEUVBnL5FnzEUm7hMAG5DUcMXANkHguIwoISpAZdFh1VhH+13HIOmxre/UN9a1l829g1dANvYWcoGJc4uUtj3HF5UKcfEmrUwISimW0Mpuin+jDlRiLvpvImqxWUyFazucpE8Kj4jqmFNnoOLAQbEerR61W1wC3fpifM9cW5mKLsSpk9uG5PUTWKA1W7u+8AgLxvfdbFA9HnDc93JKWeWyBLX6GSeVL6y9pOY9MRBHqnpPVEPcjbZ3ZpX1EPWbniF+WRCIpjcye0obTTjipWJli5HqwGGauyXPGmevCkG96jiy8nf18HrQ3459SuRSZ1lQD5EoF+1QBL/O1Y6P7PVuOSQev376RD56tOLu1EWxZAmfDNNmlZSmZSn+h5JRcjSh1NFfktIVkHtNPKw8FXDp8098oqrJ3MoNTQgE0vpXiho1QIxWhfaEU5y/WynZFk1PssjBULWNxbeIpOFYk3paNyEpb9cOkOE8ZHOdi7WWJSwHaDmx6qizOQXO75QMLIMxkCdENFx6wWbNMvKCxOlPfgkNcBaAsybM+K0AHwwvyzlcpVfEdaCexGtecBoGkjFRCG+f9InppaaSzmgbIJvkSOMUWEDO/JlFizzWAG8koM=</ds:X509Certificate>
			</ds:X509Data>
		  </ds:KeyInfo>
		</md:KeyDescriptor>
		<md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
		<md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="http://example.com/saml/acs/example"/>
	  </md:IDPSSODescriptor>
	</md:EntityDescriptor>`

	signingKeypair := &types.AsymmetricKeyPair{
		Cert:       fixtures.TLSCACertPEM,
		PrivateKey: fixtures.TLSCAKeyPEM,
	}

	encryptionKeypair := &types.AsymmetricKeyPair{
		Cert:       fixtures.EncryptionCertPEM,
		PrivateKey: fixtures.EncryptionKeyPEM,
	}

	connector, err := types.NewSAMLConnector("spongebob", types.SAMLConnectorSpecV2{
		Cert:                     signingKeypair.Cert,
		Issuer:                   "http://idp.example.com/metadata.php",
		SSO:                      "nil",
		AssertionConsumerService: "http://sp.example.com/demo1/index.php?acs",
		EntityDescriptor:         EntityDescriptor,
	})
	require.NoError(t, err)

	connector.SetSigningKeyPair(signingKeypair)
	connector.SetEncryptionKeyPair(encryptionKeypair)

	clock := clockwork.NewFakeClockAt(time.Date(2021, time.April, 4, 0, 0, 0, 0, time.UTC))
	provider, err := services.GetSAMLServiceProvider(connector, clock)
	require.NoError(t, err)
	assertionInfo, err := provider.RetrieveAssertionInfo(EncryptedResponse)
	require.NoError(t, err)
	require.NotEmpty(t, assertionInfo.Assertions)
}

// TestPingSAMLWorkaround ensures we provide required additional authn query
// parameters for Ping backends (PingOne, PingFederate, etc) when
// `provider: ping` is set.
func TestPingSAMLWorkaround(t *testing.T) {
	ctx := context.Background()
	// Create a Server instance for testing.
	c := clockwork.NewFakeClockAt(time.Now())
	b, err := lite.NewWithConfig(ctx, lite.Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            c,
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                b,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}

	a, err := NewServer(authConfig)
	require.NoError(t, err)

	// Create a new SAML connector for Ping.
	const entityDescriptor = `<md:EntityDescriptor entityID="https://auth.pingone.com/8be7412d-7d2f-4392-90a4-07458d3dee78" ID="DUp57Bcq-y4RtkrRLyYj2fYxtqR" xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata">
	<md:IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
		<md:KeyDescriptor use="signing">
		<ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
			<ds:X509Data>
			<ds:X509Certificate>MIIDejCCAmKgAwIBAgIGAXnsYbiQMA0GCSqGSIb3DQEBCwUAMH4xCzAJBgNVBAYTAlVTMRYwFAYDVQQKDA1QaW5nIElkZW50aXR5MRYwFAYDVQQLDA1QaW5nIElkZW50aXR5MT8wPQYDVQQDDDZQaW5nT25lIFNTTyBDZXJ0aWZpY2F0ZSBmb3IgQWRtaW5pc3RyYXRvcnMgZW52aXJvbm1lbnQwHhcNMjEwNjA4MTYwODE3WhcNMjIwNjA4MTYwODE3WjB+MQswCQYDVQQGEwJVUzEWMBQGA1UECgwNUGluZyBJZGVudGl0eTEWMBQGA1UECwwNUGluZyBJZGVudGl0eTE/MD0GA1UEAww2UGluZ09uZSBTU08gQ2VydGlmaWNhdGUgZm9yIEFkbWluaXN0cmF0b3JzIGVudmlyb25tZW50MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEArqJP+9QA8rzt9lLrKQigkT1HxCP5qIQH9vKgIhCDx5q7eSHOlxQ7MMa+1v1WQq1y5mgNG1zxe+cEaJ646JHQLoa0yj+rXsfCsUsKG7qceHzMR8p4y74x77PHTBJEviS9g/+fMGq7eaSK/F8ksPBfBjHnWv+lvnzrAGhxEuBXfFPf5Gb2Vr5LYurZEu9lIdFtSnFCVjzUIC1SMyovl92K4WdJpZ60N8FUSR6Jb7b8gWjnNHNc1iwr5C2b8+HUuWhqCIc0TQygEilZAdJhpYkeCQMiSqySsV+cmJ1vdjsV0HXX0YREDq6koklnw1hyTe1AckcH6qfWyBcoG2VYORjZPQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQA0eVvkB+/RSIEs7CXje7KKFGO99X7nIBNcpztp6kevxTDFHKsVlGFfl/mkksw9SjzdWSMDgGxxy6riYnScQD0FdyxaKzM0CRFfqdHf2+qVnK4GbiodqLOVp1dDE6CSQuPp7inQr+JDO/xD1WUAyMSC+ouFRdHq2O7MCYolEcyWiZoTTcch8RhLo5nqueKQfP0vaJwzAPgpXxAuabVuyrtN0BZHixO/sjjg9yup8/esCMBB/RR90PxzbI+8ZX5g1MxZZwSaXauQFyOjm5/t+JEisZf8rzrrhDd2GzWrYngB8DJLxCUK1JTM5SO/k3TqeDHLHi202P7AN2S/1CqzCaGb</ds:X509Certificate>
			</ds:X509Data>
		</ds:KeyInfo>
		</md:KeyDescriptor>
		<md:SingleLogoutService Location="https://auth.pingone.com/8be7412d-7d2f-4392-90a4-07458d3dee78/saml20/idp/slo" Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"/>
		<md:SingleLogoutService Location="https://auth.pingone.com/8be7412d-7d2f-4392-90a4-07458d3dee78/saml20/idp/slo" Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"/>
		<md:SingleSignOnService Location="https://auth.pingone.com/8be7412d-7d2f-4392-90a4-07458d3dee78/saml20/idp/sso" Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"/>
		<md:SingleSignOnService Location="https://auth.pingone.com/8be7412d-7d2f-4392-90a4-07458d3dee78/saml20/idp/sso" Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"/>
	</md:IDPSSODescriptor>
	</md:EntityDescriptor>`

	signingKeypair := &types.AsymmetricKeyPair{
		Cert:       fixtures.TLSCACertPEM,
		PrivateKey: fixtures.TLSCAKeyPEM,
	}

	encryptionKeypair := &types.AsymmetricKeyPair{
		Cert:       fixtures.EncryptionCertPEM,
		PrivateKey: fixtures.EncryptionKeyPEM,
	}

	connector, err := types.NewSAMLConnector("ping", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "https://proxy.example.com:3080/v1/webapi/saml/acs",
		Provider:                 "ping",
		Display:                  "Ping",
		AttributesToRoles: []types.AttributeMapping{
			{Name: "groups", Value: "ping-admin", Roles: []string{"admin"}},
		},
		EntityDescriptor:  entityDescriptor,
		SigningKeyPair:    signingKeypair,
		EncryptionKeyPair: encryptionKeypair,
	})
	require.NoError(t, err)

	err = a.UpsertSAMLConnector(ctx, connector)
	require.NoError(t, err)

	// Create an auth request that we can inspect.
	req, err := a.CreateSAMLAuthRequest(ctx, types.SAMLAuthRequest{
		ConnectorID: "ping",
	})
	require.NoError(t, err)

	// Parse the generated redirection URL.
	parsed, err := url.Parse(req.RedirectURL)
	require.NoError(t, err)

	require.Equal(t, "auth.pingone.com", parsed.Host)
	require.Equal(t, "/8be7412d-7d2f-4392-90a4-07458d3dee78/saml20/idp/sso", parsed.Path)

	// SigAlg and Signature must be added when `provider: ping`.
	require.NotEmpty(t, parsed.Query().Get("SigAlg"), "SigAlg is required for provider: ping")
	require.NotEmpty(t, parsed.Query().Get("Signature"), "Signature is required for provider: ping")
}

func TestServer_getConnectorAndProvider(t *testing.T) {
	ctx := context.Background()
	// Create a Server instance for testing.
	c := clockwork.NewFakeClockAt(time.Now())
	b, err := lite.NewWithConfig(ctx, lite.Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            c,
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                b,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}

	a, err := NewServer(authConfig)
	require.NoError(t, err)

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tlsCert, err := tlsca.GenerateSelfSignedCAWithSigner(
		caKey,
		pkix.Name{
			CommonName:   "server1",
			Organization: []string{"server1"},
		}, nil, defaults.CATTL)
	require.NoError(t, err)
	require.NotNil(t, tlsCert)

	keyPEM, certPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{
		Organization: []string{"Teleport OSS"},
		CommonName:   "teleport.localhost.localdomain",
	}, nil, 10*365*24*time.Hour)
	require.NoError(t, err)

	request := types.SAMLAuthRequest{
		ID:               "ABC",
		ConnectorID:      "zzz",
		CheckUser:        false,
		PublicKey:        nil,
		CertTTL:          0,
		CreateWebSession: false,
		SSOTestFlow:      true,
		ConnectorSpec: &types.SAMLConnectorSpecV2{
			Issuer:                   "test",
			Audience:                 "test",
			ServiceProviderIssuer:    "test",
			SSO:                      "test",
			Cert:                     string(tlsCert),
			AssertionConsumerService: "test",
			AttributesToRoles: []types.AttributeMapping{{
				Name:  "foo",
				Value: "bar",
				Roles: []string{"baz"},
			}},
			SigningKeyPair: &types.AsymmetricKeyPair{
				PrivateKey: string(keyPEM),
				Cert:       string(certPEM),
			},
		},
	}

	connector, provider, err := a.getSAMLConnectorAndProvider(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, connector)
	require.NotNil(t, provider)

	expectedConnector := &types.SAMLConnectorV2{Kind: "saml", Version: "v2", Metadata: types.Metadata{Name: "zzz", Namespace: apidefaults.Namespace}, Spec: *request.ConnectorSpec}
	require.Equal(t, expectedConnector, connector)

	require.Equal(t, "test", provider.IdentityProviderSSOURL)
	require.Equal(t, "test", provider.IdentityProviderIssuer)
	require.Equal(t, "test", provider.AssertionConsumerServiceURL)
	require.Equal(t, "test", provider.ServiceProviderIssuer)

	conn, err := types.NewSAMLConnector("foo", types.SAMLConnectorSpecV2{
		Issuer:                   "test",
		SSO:                      "test",
		Cert:                     string(tlsCert),
		AssertionConsumerService: "test",
		AttributesToRoles: []types.AttributeMapping{{
			Name:  "foo",
			Value: "bar",
			Roles: []string{"baz"},
		}},
	})
	require.NoError(t, err)

	err = a.UpsertSAMLConnector(ctx, conn)
	require.NoError(t, err)

	request2 := types.SAMLAuthRequest{
		ID:          "ABC",
		ConnectorID: "foo",
		SSOTestFlow: false,
	}

	connector, provider, err = a.getSAMLConnectorAndProvider(context.Background(), request2)
	require.NoError(t, err)
	require.NotNil(t, connector)
	require.NotNil(t, provider)

}

func TestServer_ValidateSAMLResponse(t *testing.T) {
	ctx := context.Background()
	// Create a Server instance for testing.
	c := clockwork.NewFakeClockAt(time.Date(2022, 04, 25, 9, 0, 0, 0, time.UTC))
	b, err := lite.NewWithConfig(context.Background(), lite.Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            c,
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                b,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}

	a, err := NewServer(authConfig)
	require.NoError(t, err)

	a.SetClock(c)

	// empty response gives error.
	response, err := a.ValidateSAMLResponse(context.Background(), "")
	require.Nil(t, response)
	require.Error(t, err)

	// create role referenced in request.
	role, err := types.NewRole("access", types.RoleSpecV5{
		Allow: types.RoleConditions{
			Logins: []string{"dummy"},
		},
	})
	require.NoError(t, err)
	err = a.CreateRole(role)
	require.NoError(t, err)

	// real response from Okta
	respOkta := `<?xml version="1.0" encoding="UTF-8"?><saml2p:Response Destination="https://boson.tener.io:3080/v1/webapi/saml/acs" ID="id336368461455218662129342736" InResponseTo="_4f256462-6c2d-466d-afc0-6ee36602b6f2" IssueInstant="2022-04-25T08:55:18.710Z" Version="2.0" xmlns:saml2p="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:xs="http://www.w3.org/2001/XMLSchema"><saml2:Issuer Format="urn:oasis:names:tc:SAML:2.0:nameid-format:entity" xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion">http://www.okta.com/exk14fxcpjuKMcor30h8</saml2:Issuer><ds:Signature xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:SignedInfo><ds:CanonicalizationMethod Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#"/><ds:SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/><ds:Reference URI="#id336368461455218662129342736"><ds:Transforms><ds:Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"/><ds:Transform Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#"><ec:InclusiveNamespaces PrefixList="xs" xmlns:ec="http://www.w3.org/2001/10/xml-exc-c14n#"/></ds:Transform></ds:Transforms><ds:DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/><ds:DigestValue>uBRfvYvl5C/LPCh36uAmRLHW76+aDP3ngChtIwP3/Fc=</ds:DigestValue></ds:Reference></ds:SignedInfo><ds:SignatureValue>M1VfkOOBH6r7niHhfGvf4OJ1HH5QJl83aD/b+mTDUUnXzHXgXlkb0BGQkSFn6ixojwCoXchpxCNzVLPN/tvfyY1dxP4MO8b+/07bGuVD2yTNlhN43/FFcDpmZ1ZDW8w2nPF1E5gy1lR8Wx2NgT3kQ2Ui1vRNX/KeX/P9NnABj4AjcshyHK2e49WLM/D4U84XOl7ODtzS7PTvtB0SGIwRE25G//8AsAv81eBfHL54Nz1HAqinMhxQtz32ZDXpKaAV6GypyBTvk6vo7Pkk4OiL6G9VIGC8Bd/gnavsc+Ickfuo7KTq8NDKTLB5WG34XKJqq6dGopSMrxr67oYjCEDZfw==</ds:SignatureValue><ds:KeyInfo><ds:X509Data><ds:X509Certificate>MIIDpDCCAoygAwIBAgIGAX4zyofpMA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG
A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
MBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW
DWluZm9Ab2t0YS5jb20wHhcNMjIwMTA3MDkwNTU4WhcNMzIwMTA3MDkwNjU4WjCBkjELMAkGA1UE
BhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV
BAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ
KoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
xQz+tLD5cNlOBfdohHvqNWIfC13OCSnUAe20qA0K8y+jtZrpwjtjjLX8iRuCx8dYc/nd6zYOhhSq
2sLmrRa09wUXXTgnLGcj50gePTaroYLyF4FNgQWLvPHJk0FGcx6JvD6L+V5RzYwH87Fhg8niP4LZ
EBw3iZnsIJN9KOuLuQeXTW0PIlMFzpCwT9aUCHCoLepe5Ou8oi8XcOCmsOESHPchV2RC/xQDIqRP
Lp1Sf7NNJ6mTmP2gOoLwsz95beOLrEI+PI/GgZBqM3OutWA0L9mAbJK9T5dPAvhnwCV+SK2HvicJ
T8c6uJxuKmoWv1t3SyaN0cIbmw6vj9CIf4DTwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCWGgLL
f3tgUZRGjmR5iiKeOeaEWG/eaF1nfenVfSaWT9ckimcXyLCY/P7CXEBiioVrxjky07iceJpi4rVE
RcVZ8SGXCa0NroESmIFlIHez6vRTrqUsfDmidxsSCwY02eaBq+9gK5iXV5WeXMKbn0yeGwF+3PkU
RAH1HuypwMH0FJRLIdW36pw7FCrGrXpk3UC6mEumXC9FptjSK1FlW+ZckgDprePOoUpypEygr2UC
XXOsqT0dwBUUttdOQMZHqIiXS5VPJ8zhYPHBGYI8WGk5FWVuXIXhgRm7LN/EyXIvCOFmDH0tVnQL
V115UGOwvjOOxmOFbYBn865SHgMndFtr</ds:X509Certificate></ds:X509Data></ds:KeyInfo></ds:Signature><saml2p:Status xmlns:saml2p="urn:oasis:names:tc:SAML:2.0:protocol"><saml2p:StatusCode Value="urn:oasis:names:tc:SAML:2.0:status:Success"/></saml2p:Status><saml2:Assertion ID="id33636846145688909913681942" IssueInstant="2022-04-25T08:55:18.710Z" Version="2.0" xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion" xmlns:xs="http://www.w3.org/2001/XMLSchema"><saml2:Issuer Format="urn:oasis:names:tc:SAML:2.0:nameid-format:entity" xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion">http://www.okta.com/exk14fxcpjuKMcor30h8</saml2:Issuer><ds:Signature xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:SignedInfo><ds:CanonicalizationMethod Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#"/><ds:SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/><ds:Reference URI="#id33636846145688909913681942"><ds:Transforms><ds:Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"/><ds:Transform Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#"><ec:InclusiveNamespaces PrefixList="xs" xmlns:ec="http://www.w3.org/2001/10/xml-exc-c14n#"/></ds:Transform></ds:Transforms><ds:DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/><ds:DigestValue>XwJSotSzU2qLdzu/WDk8dpQ/Cy1Id88932S/95+N+Ds=</ds:DigestValue></ds:Reference></ds:SignedInfo><ds:SignatureValue>qyIvGi1+w93AdGUj0+T5RYAq+CAjLSScMTMc7dLTEze6qr3mP51W/bCoZz8E47lpsbLeh0EiATa6h2Uaj6/34rILfCt3aQRNjNicu0gBKhePyNraapdnoyeqJEV8UrAOOKFiH30e5AvQ1nRZqfgY7KMt6cZH5/eXjUS63lPJJn4yr9vLw9loCdHCoHlaseh2IHi7CickyyxSMTX+Y58zpBy2g/KwN3K4oZM4a10ZYWkZpzkZJXDRSUkEc/wTTO7IPPY7Zv7R7UC+zjf5Px1sYeKTkkIxlZViZmtqjYuhibnTmhroJx7wX/LtOPxCkwLHlQRDACBNbP/UtrudU1ZMxA==</ds:SignatureValue><ds:KeyInfo><ds:X509Data><ds:X509Certificate>MIIDpDCCAoygAwIBAgIGAX4zyofpMA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG
A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
MBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW
DWluZm9Ab2t0YS5jb20wHhcNMjIwMTA3MDkwNTU4WhcNMzIwMTA3MDkwNjU4WjCBkjELMAkGA1UE
BhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV
BAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ
KoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
xQz+tLD5cNlOBfdohHvqNWIfC13OCSnUAe20qA0K8y+jtZrpwjtjjLX8iRuCx8dYc/nd6zYOhhSq
2sLmrRa09wUXXTgnLGcj50gePTaroYLyF4FNgQWLvPHJk0FGcx6JvD6L+V5RzYwH87Fhg8niP4LZ
EBw3iZnsIJN9KOuLuQeXTW0PIlMFzpCwT9aUCHCoLepe5Ou8oi8XcOCmsOESHPchV2RC/xQDIqRP
Lp1Sf7NNJ6mTmP2gOoLwsz95beOLrEI+PI/GgZBqM3OutWA0L9mAbJK9T5dPAvhnwCV+SK2HvicJ
T8c6uJxuKmoWv1t3SyaN0cIbmw6vj9CIf4DTwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCWGgLL
f3tgUZRGjmR5iiKeOeaEWG/eaF1nfenVfSaWT9ckimcXyLCY/P7CXEBiioVrxjky07iceJpi4rVE
RcVZ8SGXCa0NroESmIFlIHez6vRTrqUsfDmidxsSCwY02eaBq+9gK5iXV5WeXMKbn0yeGwF+3PkU
RAH1HuypwMH0FJRLIdW36pw7FCrGrXpk3UC6mEumXC9FptjSK1FlW+ZckgDprePOoUpypEygr2UC
XXOsqT0dwBUUttdOQMZHqIiXS5VPJ8zhYPHBGYI8WGk5FWVuXIXhgRm7LN/EyXIvCOFmDH0tVnQL
V115UGOwvjOOxmOFbYBn865SHgMndFtr</ds:X509Certificate></ds:X509Data></ds:KeyInfo></ds:Signature><saml2:Subject xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion"><saml2:NameID Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress">ops@gravitational.io</saml2:NameID><saml2:SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer"><saml2:SubjectConfirmationData InResponseTo="_4f256462-6c2d-466d-afc0-6ee36602b6f2" NotOnOrAfter="2022-04-25T09:00:18.711Z" Recipient="https://boson.tener.io:3080/v1/webapi/saml/acs"/></saml2:SubjectConfirmation></saml2:Subject><saml2:Conditions NotBefore="2022-04-25T08:50:18.711Z" NotOnOrAfter="2022-04-25T09:00:18.711Z" xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion"><saml2:AudienceRestriction><saml2:Audience>https://boson.tener.io:3080/v1/webapi/saml/acs</saml2:Audience></saml2:AudienceRestriction></saml2:Conditions><saml2:AuthnStatement AuthnInstant="2022-04-25T08:03:11.779Z" SessionIndex="_4f256462-6c2d-466d-afc0-6ee36602b6f2" xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion"><saml2:AuthnContext><saml2:AuthnContextClassRef>urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport</saml2:AuthnContextClassRef></saml2:AuthnContext></saml2:AuthnStatement><saml2:AttributeStatement xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion"><saml2:Attribute Name="username" NameFormat="urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified"><saml2:AttributeValue xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="xs:string">ops@gravitational.io</saml2:AttributeValue></saml2:Attribute><saml2:Attribute Name="groups" NameFormat="urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified"><saml2:AttributeValue xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="xs:string">Everyone</saml2:AttributeValue><saml2:AttributeValue xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="xs:string">okta-admin</saml2:AttributeValue><saml2:AttributeValue xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="xs:string">okta-dev</saml2:AttributeValue></saml2:Attribute></saml2:AttributeStatement></saml2:Assertion></saml2p:Response>`

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tlsCert, err := tlsca.GenerateSelfSignedCAWithSigner(
		caKey,
		pkix.Name{
			CommonName:   "server1",
			Organization: []string{"server1"},
		}, nil, defaults.CATTL)
	require.NoError(t, err)
	require.NotNil(t, tlsCert)

	keyPEM, certPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{
		Organization: []string{"Teleport OSS"},
		CommonName:   "teleport.localhost.localdomain",
	}, nil, 10*365*24*time.Hour)
	require.NoError(t, err)

	conn, err := types.NewSAMLConnector("saml-test-conn", types.SAMLConnectorSpecV2{
		Issuer:                   "test",
		SSO:                      "test",
		Cert:                     string(tlsCert),
		AssertionConsumerService: "test",
		AttributesToRoles: []types.AttributeMapping{{
			Name:  "foo",
			Value: "bar",
			Roles: []string{"baz"},
		}},
		SigningKeyPair: &types.AsymmetricKeyPair{
			PrivateKey: string(keyPEM),
			Cert:       string(certPEM),
		},
	})
	require.NoError(t, err)

	err = a.UpsertSAMLConnector(ctx, conn)
	require.NoError(t, err)

	err = a.Identity.CreateSAMLAuthRequest(ctx, types.SAMLAuthRequest{
		ID:                "_4f256462-6c2d-466d-afc0-6ee36602b6f2",
		ConnectorID:       "saml-test-conn",
		SSOTestFlow:       true,
		RedirectURL:       "https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml?SAMLRequest=lFZZk6JKE%2F0rHc6jYbOIiMbtiSgWEREEQVBfbrAUUMiiFFLgr%2F9i7Jm%2BPXf75j5mUifz5MmMJH%2FDQVlcl%2BDeZtUe3u4Qty99WVR4%2BfzwNro31bIOMMLLKighXrbR0gHGdsm%2B0strU7d1VBejT5B%2FRwQYw6ZFdTV60eS30e9cws54jmcnfMTGE47n40mQRPSEh3DK8zQb8gk7evFgg1FdvY3YV3r0Yn3PKqIqRlX67wnD90d4uXZda2LtHHf0An6QkOoK30vYOLDpUAQP%2B%2B3bKGvbK15SVFjjunptYQWbV1Qvp7RAUx1DERgGV0R9q5QKIjx60TC%2BQ63CbVC1byOWZtkJzU3YmUsLy9lsyQjn0YsMcYuqoH3W8CNBDLuJwEynM%2B61vrTBtYEdguQ1qksquF4%2Fff790jwG%2FGjrBOM6ht3vDAX7C8MlfXTN77oR1c2UzgQK4%2FrJa%2FT12dTlk1nz9b8V9Bv1GftbjJcOSqugvTfwe5Nj%2FF7DkqIIIa9k%2Blo3KcXSNE3RC6ovixij9MvoAwtjrUrqpykFVV2hKCjQ4ymGAdusjl9AkdYNarPyHwLzFMN%2BCzyJGK5imBH1M69fjMJQNPeD3qSsG%2FilwcEEZwE747%2BH3MMENrCK4Mthr72NvvzaeD6hbhNUOKmbEv9s%2Fl9aP6kGqw4W9RXGE%2Fyjuu%2FUfj3g36hF%2FZWgjFKI2%2F8oHayiLz8J9h7FC4o7%2FGogv2z54Iyj8bXj7FAoDc5Dwf3O0atwvsq3Ju%2FKBm0Bcnh7MvoMfjo%2B5H83%2FzQ8H%2F1%2BR2wJqAs5PUmLBA%2B5pPtrwRn4rhbER6QzrNHr9h1nztqm1S0kJT1Oo0NfxHRdnvx%2B27Lurbdl8FgdEZvM3FBqnV1DeHisSX7ezTHT24DMvdJBpb%2FlrVj0UmQk%2BVHhB1MyU65CSl2t1cHyBANokpgwPbrm86ne%2BBssK0UEVoAqlKFREkvel%2BWM7Qkv7MVOsUg0X%2FM3Yc1x2prVjXw4CUVxccn6UO53BsObdG4MpcBNGcYaWyp9NBQXcRTKtqSdVpyo7zM7PA6OQ53nJ0%2BMTGtqHpFmVLfxdnaKUpMa32Wt2N%2Bsy0NYr8aC7gxb5KPAnR%2Flta%2B0VaWa8I5XdyHSZ3XZiMM4Ho9tKaHA29uH9J%2B0%2Fia%2FDoePVhxn9EIO2uDDkL7t0wRFQQu%2FGpom6w9JAtkuBUQTQartwVbT871EMqWEfmR2ZGASUToQ2T5t9PqsZV1kAlvZijYgtqx4hmiogDkoUmYYnurh81HsospOXcZ0DSciG%2Fske7YtK%2F2MPvt9EamLIZZmOGTNLFSzLpra91Bd5Ce%2Fv4QskwU%2BR9ZZZBq5RkxZYww5ZYyHwvnffI%2Bnb%2Fjw5SIw9geikGcOXSH94Y8conVgjH7zAIWYmp4IDHdd7YtQ9Ug43dDbsu9O7AoH6uLxB599F%2Fqra5hLEnC0P9csijaQ01SxgCxJwK6lNFVEYAZRd7v7M%2BFkb1nIzriDojbZoOe3vb2z8xUppklq%2BWaQ9tNq84j53vOPQqEG7KbjFFo93MIT2GoBVXgNOCVuy9oqdDirWoiHq6pTV9NKdySK52MZjvO5hjMLye6miZrHoqR7nN96vqwyTKsb%2FZTz7W63urT12CWsdlmV05vNRw6rcpcdjw26i3w%2FgNhShGyfn%2BVNfM%2Bgf7xk1IWiu16cx7QVxoXZFJej1yyctR%2FHM51Hjqe4xgy4sraoSXSSbuJj8O%2F0GMw4xO38u5s6F3Z6m7GPqddGEb3SRKII6%2FMBGHGHUkd30Km51Xi%2FSFhm3%2FN%2BRBToqKs5ZeJzCQKj7u1wk%2FYL9mblYsBeBAkQBYDASA2RI3J6kr09bQF7TYnAlkEKgQHIt7mLFaKIFLElA4C%2Fm1H52SP56MXrtT1di%2FxxbqOHV9mUlKYPZacgQ2KkAveXIzxXIrPm3etKKRd5787nrSeqIt0vYjAopNyRE0iT8WrmbrSCL2%2FJoIVHfXxc9bcgquX1tWmt9lbe1ky7Z2l1EVlC2SdhOQ5v3LzZ%2BSbTpqDUir4P9vyZVV2Ffehk1g15fMYOiqX%2BEixOsS%2BAVImuldfCW583mHN2UYjijYbHC7PqW3DdWIa%2B2l3CS5dv%2BtkCXxIJm1VzEeSpBJGM3Dm1NuStwUXCVHFVmpMo52E6nVpcE54uXS2%2FDI8hBnqnIR2hBWZLRkfuDqu6d5GVZmyFRl1zA3%2Fci3Tr1WUs2f1DOm9lzCPSIVU9wGZOtPcF9Oel8uF8XzvU54X008L6%2Bv2gNYMSarJVFygaXkBR1ERqYNDCt1Hb3OHoZVU3ZdD%2B8%2B3IvDJPD4onyfPp8l7hK4xQgmD8%2FKf%2B9XD%2B%2Br8AAAD%2F%2Fw%3D%3D",
		ClientRedirectURL: "http://127.0.0.1:57293/callback?secret_key=70e98f8871530e66e6a136ae71fe002454dc1c76d754f090f895508a2226c36b",
		ConnectorSpec: &types.SAMLConnectorSpecV2{
			Issuer:                   "http://www.okta.com/exk14fxcpjuKMcor30h8",
			SSO:                      "https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml",
			Cert:                     "",
			Display:                  "Okta",
			AssertionConsumerService: "https://boson.tener.io:3080/v1/webapi/saml/acs",
			Audience:                 "https://boson.tener.io:3080/v1/webapi/saml/acs",
			ServiceProviderIssuer:    "https://boson.tener.io:3080/v1/webapi/saml/acs",
			EntityDescriptor:         "\u003c?xml version=\"1.0\" encoding=\"UTF-8\"?\u003e\u003cmd:EntityDescriptor entityID=\"http://www.okta.com/exk14fxcpjuKMcor30h8\" xmlns:md=\"urn:oasis:names:tc:SAML:2.0:metadata\"\u003e\u003cmd:IDPSSODescriptor WantAuthnRequestsSigned=\"false\" protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:2.0:protocol\"\u003e\u003cmd:KeyDescriptor use=\"signing\"\u003e\u003cds:KeyInfo xmlns:ds=\"http://www.w3.org/2000/09/xmldsig#\"\u003e\u003cds:X509Data\u003e\u003cds:X509Certificate\u003eMIIDpDCCAoygAwIBAgIGAX4zyofpMA0GCSqGSIb3DQEBCwUAMIGSMQswCQYDVQQGEwJVUzETMBEG\nA1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU\nMBIGA1UECwwLU1NPUHJvdmlkZXIxEzARBgNVBAMMCmRldi04MTMzNTQxHDAaBgkqhkiG9w0BCQEW\nDWluZm9Ab2t0YS5jb20wHhcNMjIwMTA3MDkwNTU4WhcNMzIwMTA3MDkwNjU4WjCBkjELMAkGA1UE\nBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNV\nBAoMBE9rdGExFDASBgNVBAsMC1NTT1Byb3ZpZGVyMRMwEQYDVQQDDApkZXYtODEzMzU0MRwwGgYJ\nKoZIhvcNAQkBFg1pbmZvQG9rdGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA\nxQz+tLD5cNlOBfdohHvqNWIfC13OCSnUAe20qA0K8y+jtZrpwjtjjLX8iRuCx8dYc/nd6zYOhhSq\n2sLmrRa09wUXXTgnLGcj50gePTaroYLyF4FNgQWLvPHJk0FGcx6JvD6L+V5RzYwH87Fhg8niP4LZ\nEBw3iZnsIJN9KOuLuQeXTW0PIlMFzpCwT9aUCHCoLepe5Ou8oi8XcOCmsOESHPchV2RC/xQDIqRP\nLp1Sf7NNJ6mTmP2gOoLwsz95beOLrEI+PI/GgZBqM3OutWA0L9mAbJK9T5dPAvhnwCV+SK2HvicJ\nT8c6uJxuKmoWv1t3SyaN0cIbmw6vj9CIf4DTwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCWGgLL\nf3tgUZRGjmR5iiKeOeaEWG/eaF1nfenVfSaWT9ckimcXyLCY/P7CXEBiioVrxjky07iceJpi4rVE\nRcVZ8SGXCa0NroESmIFlIHez6vRTrqUsfDmidxsSCwY02eaBq+9gK5iXV5WeXMKbn0yeGwF+3PkU\nRAH1HuypwMH0FJRLIdW36pw7FCrGrXpk3UC6mEumXC9FptjSK1FlW+ZckgDprePOoUpypEygr2UC\nXXOsqT0dwBUUttdOQMZHqIiXS5VPJ8zhYPHBGYI8WGk5FWVuXIXhgRm7LN/EyXIvCOFmDH0tVnQL\nV115UGOwvjOOxmOFbYBn865SHgMndFtr\u003c/ds:X509Certificate\u003e\u003c/ds:X509Data\u003e\u003c/ds:KeyInfo\u003e\u003c/md:KeyDescriptor\u003e\u003cmd:NameIDFormat\u003eurn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress\u003c/md:NameIDFormat\u003e\u003cmd:SingleSignOnService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Location=\"https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml\"/\u003e\u003cmd:SingleSignOnService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect\" Location=\"https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml\"/\u003e\u003c/md:IDPSSODescriptor\u003e\u003c/md:EntityDescriptor\u003e",
			EntityDescriptorURL:      "",
			AttributesToRoles: []types.AttributeMapping{
				{
					Name:  "groups",
					Value: "okta-admin",
					Roles: []string{"access"},
				},
			},
			SigningKeyPair: &types.AsymmetricKeyPair{
				PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA1py+q5bnxhAvZ7bnhQQauHIqOpFA5CMXCXd+A9Y1qDHecnN3\nrFVZfyUZrYm/gTQZSptgAshr+VWsBh9O3ZAZ5Lg+f0FSkYr+k0+A7Bx3v4N76Psi\nyE+INMmtyvP2bTGyOrHqaeGzQYkpiPq044WS2j5PDYiQWbepDpxLYbiQ7qwzS9xZ\nZp6w8TyFGNkMl26F5ZeSH+T/S/EHt3Q9t2U2uWRdWv1IdZ13krqJJURMzkBMMj2j\nBxgKoHPJa7T4DniLg5a5OBKTbernbPdW1xzQUgHATwdlQAx2+KBIpKJiuqixH1/b\nVHHpZzAR5IYXv82xmYBoyjFBsmDH3ao+MFraTwIDAQABAoIBAQCEXZbINEHtiiwC\n1u/Cvb5RRrC/ALm6O95Ii3egnCzp+SAPDSKhmt6hKdvFifEgmmaC+oPkE4Ns/Ccm\ne4bj5q3hwLVjPYHUnJrZdq64cfJ1n338O3C/hTYoAL/9Li0uOfmIdBV1iqxJ3nRM\ntPx+W/MwQj/1w+XsP/e4ODPSKMjTOyZkLVhArLA2qBM3l1NBWQw4EV96m1Dvjq2o\nxFYhSODZOYDYXq82NZya3cBzj30kEB/6fNk6qPAsMa3Ck7F/9mx3MA2XM/S0aB/U\nq+5I1g+mTAMPKa2c2Tv2hwWuRU9ddKGiXuw/gHPoBwEU7AVNk+nNSVDhj5/pqcFB\nM/lPNg6xAoGBAPu/oICyXwlXYfsvkTx+HHpY7Imq8RtBUjVpD3z+LC6+QydlF2Z/\nNqLDxBPZAAdA7VGzw5QdNR8DKY9EgeRQAcci8FIqjueTPQDVKl9kwJzOxqg8DM8t\nR8YpIOtP22JvjHAkFafBq9cWYZNLQwVabIkCdc0jUruN53WKRkj+b0npAoGBANo8\nkX7ypsS+riLu6Ez89tXHV78CW8eMb/Wxsa2hgrzd7KmgjYdj2wlfRaEY6pSvBA1Y\nMvy/Emvm2KhwuGzWWPJvoaQq/Hr6Puns9DVP8U3TTJDC3R5dDZUQ0DiVUGyez9Qu\nXV5aNMnXFjGPdRQYjGM1zG6677dM0CVYu/6MVSd3AoGBAN5X3tALufgsHzOUTXfa\nAhjk1PS573yc8piNk8pXSnp2PCVdGY/DJ2QV9uV4sJe3dmLEnCYCrdoYFuqcHQSi\nzQ8uAobvY4uP9T75BhV+jMdxsO8BKmcInO2dgZ+SxjZoQucAV8f0O2saL0/CFw1x\nUY6oh5aIbheMOzMKzwzE+1GRAoGAYm5FFWPuUfjK49irj+XckulZKz6uFJ/D86YU\nxIJ/TB4wWwWeL/2a0mxVJGbvjuYtRrOMM7EeZup0t+w3UmePMLGmzzvQKstpyupj\n7xPCe16dPwGU59gCg0RVFeBKqOMsS8Apvp+jBZJsYSgaH1k/IJQoQ50u95a+nsmZ\n6SJ0WdsCgYBfTcIJ456LNPQ7sjDtcqIQcbBgXYIt/u4dIhz0zuLSfvTrXAtcy7nU\nEDU+N2Ay715GmS+/iwc592Itam93t3sl1ql/Y+SrrxGQ7zRd6MALKIxM0+4qptB5\nDFXT1B5qhKHdZFr71AIGPrUjfsRQV8tPKFjRkuQ0zqEPvi4g06RMRw==\n-----END RSA PRIVATE KEY-----\n",
				Cert:       "-----BEGIN CERTIFICATE-----\nMIIDKzCCAhOgAwIBAgIRALIKjRCwhEmeWcNvwy1fBCUwDQYJKoZIhvcNAQELBQAw\nQDEVMBMGA1UEChMMVGVsZXBvcnQgT1NTMScwJQYDVQQDEx50ZWxlcG9ydC5sb2Nh\nbGhvc3QubG9jYWxkb21haW4wHhcNMjIwNDI1MDg1MzE4WhcNMzIwNDIyMDg1MzE4\nWjBAMRUwEwYDVQQKEwxUZWxlcG9ydCBPU1MxJzAlBgNVBAMTHnRlbGVwb3J0Lmxv\nY2FsaG9zdC5sb2NhbGRvbWFpbjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC\nggEBANacvquW58YQL2e254UEGrhyKjqRQOQjFwl3fgPWNagx3nJzd6xVWX8lGa2J\nv4E0GUqbYALIa/lVrAYfTt2QGeS4Pn9BUpGK/pNPgOwcd7+De+j7IshPiDTJrcrz\n9m0xsjqx6mnhs0GJKYj6tOOFkto+Tw2IkFm3qQ6cS2G4kO6sM0vcWWaesPE8hRjZ\nDJduheWXkh/k/0vxB7d0PbdlNrlkXVr9SHWdd5K6iSVETM5ATDI9owcYCqBzyWu0\n+A54i4OWuTgSk23q52z3Vtcc0FIBwE8HZUAMdvigSKSiYrqosR9f21Rx6WcwEeSG\nF7/NsZmAaMoxQbJgx92qPjBa2k8CAwEAAaMgMB4wDgYDVR0PAQH/BAQDAgeAMAwG\nA1UdEwEB/wQCMAAwDQYJKoZIhvcNAQELBQADggEBADXVdHHQ3HB6X7QizVnQ/Cgg\nzEOEiMC1ClsxkXeZnB1H6TpFEm9jxT77tVBGB0x9dAyEwmOwYAgf+F5TJIl6mqfy\nIbXK+XFxqacoDHprtPtqmqH1tR20G9cP8mxfbm+bq47rOWN1tgAmIlxxaR6Z2GTE\n2zKw5vyjdZsSidCxka9YdW8AgEcpnVteqxjrs4SOcbidJIs+9NnxtApJPMKFOkbk\nvjJx59skfCsNnrk8D3CeiDiT7/HMDLM4c83ETG04C/SzNSvGlpf60mTIjkyzydAK\nvIiKii9s2m1KiTOsGKVkDEr+PbMoo4y6XRB0tVomdCQxzCZLDs6iwviGGUer7wI=\n-----END CERTIFICATE-----\n",
			},
			Provider:          "",
			EncryptionKeyPair: nil,
		},
	}, defaults.SAMLAuthRequestTTL)
	require.NoError(t, err)

	// check ValidateSAMLResponse
	response, err = a.ValidateSAMLResponse(context.Background(), base64.StdEncoding.EncodeToString([]byte(respOkta)))
	require.NoError(t, err)
	require.NotNil(t, response)

	// check internal method, validate diagnostic outputs.
	diagCtx := a.newSSODiagContext(types.KindSAML)
	auth, err := a.validateSAMLResponse(context.Background(), diagCtx, base64.StdEncoding.EncodeToString([]byte(respOkta)))
	require.NoError(t, err)

	// ensure diag info got stored and is identical.
	infoFromBackend, err := a.GetSSODiagnosticInfo(context.Background(), types.KindSAML, auth.Req.ID)
	require.NoError(t, err)
	require.Equal(t, &diagCtx.info, infoFromBackend)

	// verify values
	require.Equal(t, "ops@gravitational.io", auth.Username)
	require.Equal(t, "ops@gravitational.io", auth.Identity.Username)
	require.Equal(t, "saml-test-conn", auth.Identity.ConnectorID)
	require.Equal(t, "_4f256462-6c2d-466d-afc0-6ee36602b6f2", auth.Req.ID)
	require.Equal(t, 0, len(auth.HostSigners))

	authnInstant := time.Date(2022, 04, 25, 8, 3, 11, 779000000, time.UTC)

	// ignore, this is boring and very complex.
	require.NotNil(t, diagCtx.info.SAMLAssertionInfo.Assertions)
	diagCtx.info.SAMLAssertionInfo.Assertions = nil

	require.Equal(t, types.SSODiagnosticInfo{
		TestFlow: true,
		Error:    "",
		Success:  true,
		CreateUserParams: &types.CreateUserParams{
			ConnectorName: "saml-test-conn",
			Username:      "ops@gravitational.io",
			Roles:         []string{"access"},
			Traits: map[string][]string{
				"groups":   {"Everyone", "okta-admin", "okta-dev"},
				"username": {"ops@gravitational.io"},
			},
			SessionTTL: 108000000000000,
		},
		SAMLAttributesToRoles: []types.AttributeMapping{
			{
				Name:  "groups",
				Value: "okta-admin",
				Roles: []string{"access"},
			},
		},
		SAMLAttributesToRolesWarnings: nil,
		SAMLAttributeStatements: map[string][]string{
			"groups":   {"Everyone", "okta-admin", "okta-dev"},
			"username": {"ops@gravitational.io"},
		},
		SAMLAssertionInfo: &types.AssertionInfo{
			NameID: "ops@gravitational.io",
			Values: map[string]samltypes.Attribute{
				"groups": {
					XMLName:    xml.Name{Space: "urn:oasis:names:tc:SAML:2.0:assertion", Local: "Attribute"},
					Name:       "groups",
					NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
					Values: []samltypes.AttributeValue{
						{
							XMLName: xml.Name{Space: "urn:oasis:names:tc:SAML:2.0:assertion", Local: "AttributeValue"},
							Value:   "Everyone",
						},
						{
							XMLName: xml.Name{Space: "urn:oasis:names:tc:SAML:2.0:assertion", Local: "AttributeValue"},
							Value:   "okta-admin",
						},
						{
							XMLName: xml.Name{Space: "urn:oasis:names:tc:SAML:2.0:assertion", Local: "AttributeValue"},
							Value:   "okta-dev",
						},
					},
				},
				"username": {
					XMLName:    xml.Name{Space: "urn:oasis:names:tc:SAML:2.0:assertion", Local: "Attribute"},
					Name:       "username",
					NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
					Values: []samltypes.AttributeValue{
						{
							XMLName: xml.Name{Space: "urn:oasis:names:tc:SAML:2.0:assertion", Local: "AttributeValue"},
							Value:   "ops@gravitational.io",
						},
					},
				},
			},
			WarningInfo:                &saml2.WarningInfo{},
			SessionIndex:               "_4f256462-6c2d-466d-afc0-6ee36602b6f2",
			AuthnInstant:               &authnInstant,
			SessionNotOnOrAfter:        nil,
			Assertions:                 nil,
			ResponseSignatureValidated: true,
		},
		SAMLTraitsFromAssertions: map[string][]string{
			"groups":   {"Everyone", "okta-admin", "okta-dev"},
			"username": {"ops@gravitational.io"},
		},
		SAMLConnectorTraitMapping: []types.TraitMapping{
			{
				Trait: "groups",
				Value: "okta-admin",
				Roles: []string{"access"},
			},
		},
	}, diagCtx.info)

	// make sure no users have been created.
	users, err := a.GetUsers(false)
	require.NoError(t, err)
	require.Equal(t, 0, len(users))
}
