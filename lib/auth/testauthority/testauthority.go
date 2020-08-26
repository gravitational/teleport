/*
Copyright 2015 Gravitational, Inc.

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

package testauthority

import (
	"crypto/rand"
	random "math/rand"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/wrappers"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

type Keygen struct {
}

func New() *Keygen {
	return &Keygen{}
}

func (n *Keygen) Close() {
}

func (n *Keygen) GetNewKeyPairFromPool() ([]byte, []byte, error) {
	return n.GenerateKeyPair("")
}
func (n *Keygen) GenerateKeyPair(passphrase string) ([]byte, []byte, error) {
	randomKey := testPairs[(random.Int() % len(testPairs))]
	return randomKey.Priv, randomKey.Pub, nil
}

func (n *Keygen) GenerateHostCert(c services.HostCertParams) ([]byte, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(c.PublicHostKey)
	if err != nil {
		return nil, err
	}
	validBefore := uint64(ssh.CertTimeInfinity)
	if c.TTL != 0 {
		b := time.Now().Add(c.TTL)
		validBefore = uint64(b.Unix())
	}
	principals := native.BuildPrincipals(c.HostID, c.NodeName, c.ClusterName, c.Roles)
	principals = append(principals, c.Principals...)
	cert := &ssh.Certificate{
		ValidPrincipals: principals,
		Key:             pubKey,
		ValidBefore:     validBefore,
		CertType:        ssh.HostCert,
	}
	cert.Permissions.Extensions = make(map[string]string)
	cert.Permissions.Extensions[utils.CertExtensionRole] = c.Roles.String()
	cert.Permissions.Extensions[utils.CertExtensionAuthority] = c.ClusterName
	signer, err := ssh.ParsePrivateKey(c.PrivateCASigningKey)
	if err != nil {
		return nil, err
	}
	signer = sshutils.AlgSigner(signer, c.CASigningAlg)
	if err := cert.SignCert(rand.Reader, signer); err != nil {
		return nil, err
	}
	return ssh.MarshalAuthorizedKey(cert), nil
}

func (n *Keygen) GenerateUserCert(c services.UserCertParams) ([]byte, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(c.PublicUserKey)
	if err != nil {
		return nil, err
	}
	validBefore := uint64(ssh.CertTimeInfinity)
	if c.TTL != 0 {
		b := time.Now().Add(c.TTL)
		validBefore = uint64(b.Unix())
	}
	cert := &ssh.Certificate{
		KeyId:           c.Username,
		ValidPrincipals: c.AllowedLogins,
		Key:             pubKey,
		ValidBefore:     validBefore,
		CertType:        ssh.UserCert,
	}
	signer, err := ssh.ParsePrivateKey(c.PrivateCASigningKey)
	if err != nil {
		return nil, err
	}
	signer = sshutils.AlgSigner(signer, c.CASigningAlg)
	cert.Permissions.Extensions = map[string]string{
		teleport.CertExtensionPermitPTY:            "",
		teleport.CertExtensionPermitPortForwarding: "",
	}
	if c.PermitX11Forwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitX11Forwarding] = ""
	}
	if c.PermitAgentForwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitAgentForwarding] = ""
	}
	if !c.PermitPortForwarding {
		delete(cert.Permissions.Extensions, teleport.CertExtensionPermitPortForwarding)
	}

	// Add roles, traits, and route to cluster in the certificate extensions if
	// the standard format was requested. Certificate extensions are not included
	// legacy SSH certificates due to a bug in OpenSSH <= OpenSSH 7.1:
	// https://bugzilla.mindrot.org/show_bug.cgi?id=2387
	if c.CertificateFormat == teleport.CertificateFormatStandard {
		traits, err := wrappers.MarshalTraits(&c.Traits)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(traits) > 0 {
			cert.Permissions.Extensions[teleport.CertExtensionTeleportTraits] = string(traits)
		}
		if len(c.Roles) != 0 {
			roles, err := services.MarshalCertRoles(c.Roles)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cert.Permissions.Extensions[teleport.CertExtensionTeleportRoles] = roles
		}
		if c.RouteToCluster != "" {
			cert.Permissions.Extensions[teleport.CertExtensionTeleportRouteToCluster] = c.RouteToCluster
		}
	}
	if err := cert.SignCert(rand.Reader, signer); err != nil {
		return nil, err
	}
	return ssh.MarshalAuthorizedKey(cert), nil
}

type PreparedKeyPair struct {
	Priv []byte
	Pub  []byte
}

// for unit tests we have 4 pre-generated pub/priv keypairs
var (
	testPairs = []PreparedKeyPair{
		{
			Priv: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAvJGHcmQNWUjY2eKasmw171qZR0B5FOnzy/nAGB1JAE+QokFe
Bjo8Gkk3L2TSuVNn0NI5uo5Jwp7GYtbfSbowo11E922Bwp0sFoVzeeUMyLud9EPz
Hl8+VvE8WEa1lC4D4aqravAfTeeePrONIYoBttX5oYXQ7aZkM8N7yS7KWNOZpy9f
n1vkSCpDOK29edLHWVyiDcXzULxEbXhPFl9Ly9shuEbqic2LRggxBnh3fhy53u8X
5qj8bp+21GGsQJaZYZtc9ieNYamo/KQcA0hFfUgTmV74ehY0vZ7yQk+2dW22cFqw
Dv+xNmnNHlfuYhHNCfk8rnztxfbqHfifgCArQQIDAQABAoIBADhq8jNva+8CtJ68
BbzMU3bBjIqc550yQhcNKkQMvwKwy31AQXlrgv/6V+B+Me3w3mbD/zGp0LfB+Wkp
ELVmV5cJGNFOmjw3+jDizKHzvddxCtlCW0MDDAvHMV7YCQvEmLSz84WTQkp0ugvY
fKlEOS8S5hVFjDUOS3yRSD/xF+lrIlYUaR4gXnDAJZx9ttgfZlHOp8ehxk+1bn59
3Fv1fCXcCKmKUlTk1kFasD8P+2M3MKP42Ih5ap9cfLSVPiBS/6JRBxIlZrHM9/2a
w6vEp+qMwwgCmxLPMwZfem6LNHO/huTrWKf4ltVubb5bUXIe22udKp2WK4NWc3Ka
uG8EleECgYEA4A9Mwd0QJs0j1kpuJDNIjfFx6IROv3QAb0QPq0+192ZF8P9AEj8B
TNDQVzb/skM+2NDdvhZ5v4+OJQcUNpEskhX+5ikk8QHGAUY6vT8rO6oiIRMaxLuJ
OEDc2Qms1OmctTmgSVyaxfXIK2/GDdvOizt0Z7Y7abza4bigEm49hyMCgYEA13MI
H429Ua0tnVVmGJ/4OjnKbgtF7i02r50vDVktPruKWNy1bhRkRyaOoCH7Zt9WXF2j
GapZZN1N/clO4vf9gikH0VCo4Tc2JR635dXdfISlt8NLXmR800Ms1UCAKlwIOQjz
dgHcvEbvFwSe1MFgOJVGL82G2rUA/zDVOKdjXEsCgYAZxyjZlQlqrWdWHDIX0B6k
1gZ47d/xfvMd2gLDfuQ8lnOtinBgqQcJQ2z028sHQ11TrJQWbpeLRoTgFbRposIx
/H3bFRi+8alKND5Fz6K1tpk+nOgTglADPNMr1UUhKc9xujOKvTDBXcmt1ao/pe5Z
bnmyBPFI9QVpusgP1scVaQKBgE5mJYaV5VZbVkXyVXyQeZt2fBsfLwtEmKm+4OhS
kwxI4kcDyWGNOhBKD4xl0T3V928VA8zLGEyD22WGY5Zj93PtylJ4r3uEw8cuLm0M
LdSp0EPWZQ6sMmAOCbpwBjNj2fonL7C5bMF2bnpJzCJPW9w7NZcfivr68qnp8yzy
fE2RAoGBALWvlHVH/29KOVmM52sOk49tcyc3czjs/YANvbokiItxOB8VPY6QQQnS
/CBsCZxUuWegYmkUnstHDmY1LYqjxW4goOqizIksaReivPmsTuQ1qd+aqXTfg2pt
uy6c6X17xkP5q2Lq4i90ikyWm3Oc25aUEw48pRyK/6rABRUzpDLB
-----END RSA PRIVATE KEY-----`),
			Pub: []byte(`ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC8kYdyZA1ZSNjZ4pqybDXvWplHQHkU6fPL+cAYHUkAT5CiQV4GOjwaSTcvZNK5U2fQ0jm6jknCnsZi1t9JujCjXUT3bYHCnSwWhXN55QzIu530Q/MeXz5W8TxYRrWULgPhqqtq8B9N554+s40higG21fmhhdDtpmQzw3vJLspY05mnL1+fW+RIKkM4rb150sdZXKINxfNQvERteE8WX0vL2yG4RuqJzYtGCDEGeHd+HLne7xfmqPxun7bUYaxAlplhm1z2J41hqaj8pBwDSEV9SBOZXvh6FjS9nvJCT7Z1bbZwWrAO/7E2ac0eV+5iEc0J+TyufO3F9uod+J+AICtB`),
		},
		{
			Priv: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEA2KcFrvuCPSc+3KTm/Lc+iES+GXTJaf7yuV5hXgtj1G/Aan0k
v0jMUdpg7CN5ETBEax9zHHSXxHW1GpysfQz4nGnRwLb3KgzrhZgo2KKMvF+CnY7d
jMajtK3h9hYoZZckii03vk05Mf4fOJjd8feOOQFpI7dKNsKD5gUGqzGwno5+/lnb
F6qnGTDDqwNNvLEGwFUjDw5l4WnEZ8XGHSpFdHf8mbHSchMSh6y/BABNpgx4KX/v
9NcPJFcNN3SEWLNuGjzm0pJ97Zvh3gjxZ2mg1HB7rpYla1gb0SWaJ9KTnvXGbDEa
dH+K+kdGfe1uvOYuPDrT7BfNyc2Tz3hxEA0lgQIDAQABAoIBAQDCkdWH3baMhzds
XuhXY5ZUOTBkmj7c46tHENzu4dnJCofK2xLqe02L4UyUJhNvfWKktfziPE+kj3WT
Lcu3DrQjfOF0ap009Z97PjjIvcsYzcn3CDwuVqLk/BhnsmSbQA7/zTY3wRCxtiCB
6r/As+vVhE/RVKXg4fYk2LSxgJG3AmhWSglg0fIo9COdLmLhmnz80JMnzxXKuDNs
xb4DzaDYCpK24tb7Y0PAl28RSe0M398tLVrYNX0mfwbbfIn+B0g+6oRFBYvuac6S
OgQdVEH4qds28JWeDr+vvpS7Ogwfx7b1mFsfiguS485xtc6Pfys0IhHZuhNlf2pW
qrm3AVn1AoGBAPu3dArwZzDgtCu5PLLH32cKgWC/XpjnLKchoskQJu5A1Ug99pxI
K/bOc363d3CDq3BU9Nd6/L+dH57RvQi7PDYkHjBwGFVwvS6ozZ7eIl1mEZBrnpmD
7IrlT+m5zEyQSB6ZThgDMJY8j76pqOexdN/H/Dlh7uDFV7w23Fn2QaMzAoGBANxW
0dduRb/ut6qTj4x6D4K8nCi/50a51LvblwjkaE2n6xYGYM6pURIhDOMy1nDCDyMz
m81Si29Pa4hcnIf4Od4K9abxB4E/qzyMY6mrNRgY33cjjACHhBKLK3JarKiOmlli
El5PM/vve4cFsu4qkIvKLmf22/KJ8HpNhO1nLVR7AoGAEr9CDEKFXPWPVaZRJ/uM
3u7AXgVCtV6aS8RMjG8Ah0Qa3muG/3K8m4Aax/hAFAgqb45UQewuANNh9IEodAsF
2/5qpS7kERD5dg0qa0eeBZjBfCEXydUye9HCVuT4m0cvp9/BGja6mqXeCtQ1+TOV
QclyNo/dq63m7+SiGq0ljFMCgYEAxqFduh+msUe6OwObPMAsi2cMP5AAJjoQFOn4
VgPSI29k9g3551OryfQRch+6QRwwGUPFCGuJV2b5QYx7b/fN8uVeXoiag2GqNIM6
tRGqY3bIvNZGt5Ny9GSRXh1v2OP1MO7AMFSmQE+7xBTXIO0uMVaqTv6zeQnwx9Bq
LLn+m1ECgYEA15K07VEQ2bjwJLpLfUye9j/we/mcEHPju5HkccmiKJ2BlD8uWLaF
3ChrOtryKxEed6yhrzKRtH+CPbYSyHcgmUcsjidxJ5DKBl89bRvHXjzrBMkqKFKP
zVpCU4VorDd82D/VlJwWUZ0DKw79EldNHyi8Y4Nhv0HP8QEH4teEtnc=
-----END RSA PRIVATE KEY-----`),
			Pub: []byte(`ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDYpwWu+4I9Jz7cpOb8tz6IRL4ZdMlp/vK5XmFeC2PUb8BqfSS/SMxR2mDsI3kRMERrH3McdJfEdbUanKx9DPicadHAtvcqDOuFmCjYooy8X4Kdjt2MxqO0reH2FihllySKLTe+TTkx/h84mN3x9445AWkjt0o2woPmBQarMbCejn7+WdsXqqcZMMOrA028sQbAVSMPDmXhacRnxcYdKkV0d/yZsdJyExKHrL8EAE2mDHgpf+/01w8kVw03dIRYs24aPObSkn3tm+HeCPFnaaDUcHuuliVrWBvRJZon0pOe9cZsMRp0f4r6R0Z97W685i48OtPsF83JzZPPeHEQDSWB ekontsevoy@turing`),
		},
		{
			Priv: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAouygGWpaFGjNsyY7XUpFGbeEtXg7WKdFI4Qz/o3HmGOwUr1w
mwjOYy6tJSm11a0lntzPKYNv4x+MruoAyPEjA+UYMSHYcOWaoKcGPOZcXP8aZFV2
VgFTn7eg9uKDZhDCcEa0lB8057MEzaHZgZU0B8+TdmjF1de80awlAEGZK3KsjZqF
EDA/Frl/yNVU1Kywg9bx9EuPXsT50VVWBja8Wo5nC/TOZt8NFnrymWr+J2okV1pK
ccD160TMKm/lL5GKIIMz1Zi8oYcFrCdRV2lnckJYd/QM6wrA4cLD5lN//zwkVmqU
PY5LiY1oEpZ6BT2hH7jACjG0ZcFU+HxmhQF8awIDAQABAoIBAQCbAQMkiwFer4Mc
cUBDdlidqfLRb60OoD1wF+Qbx6nget+TKHaMmWk6BXtngvJjc1L6fFt/xHPbovWG
qEzM4FYO65QDko7IgjmFpMKTiBrRw0bJtGFcW/DCYML8f+7BWSqUBUDiN3pvAeuM
8/HqjhgtYjiKjA3EcHdNCDk/sClYokD25T9/w1cYaPaNWHQVa5o2fv5NWrU0lAH8
oLsMmhhmqDZH9HY1lw0IdxEdXlvUxwSm+w42UCVWQf0D9ph8k/ACr2zvuTBjn1Dk
B776W5z2zZGNDw+omBpuwCZkmIBLegWDSgRV/Z9AxTsim1b5mC1KwDqpvb2cXrfm
Oq7tR4chAoGBANhXV+jC94cQ1tffkmxU+K4bhosfI8zX4EZht1IcqCdf/iqupsXH
O/cw1V5MDMQdZrD27W1lJhVH3Xd2KLcoaAx8fxSHYZpet2iQe6I2Nbkcj11o9hj7
xvONzclvMgEF13Xai0OY/4HHBJMfpGxDGroDmyub1pYQYz3rULJPpbZbAoGBAMDK
fIqmm/06IZwwGTddA7/aLMjpCAlH4pNu0VntXuKPA44p1ORIdTFNE0Z+6SIHJOcj
rV1XXrNM7zTq2pFJ7sm0wOylz9Ts1ThYtXKX8WVVBxgy6TlR263dq8BFXXHvLk8E
1+W7bmisPgAIHfu4zaJc4MvIBUTlAuQjz867Ys8xAoGBAKUNfjRHCzIw1ri8Cao8
6b1roqphh56w1Jrd0k8DLgdcZT2LIhGif02IJEFdJCA7ji1VNq9PjE6QFZcevtF+
MmPUV+ABqaVsveE42hpX4YTpFTfe7GMDNDZ86ZPVEgFVw5xWsAlSoR0SCZt1eKxg
RfPE7I3Ix16WAiErdtWTjoohAoGAOewcIuQPtbMDahOhX9rYR4nbLrmkqnUog7cl
uujwOw1QuiOjTLrgSuGnSuTSUmDnG3LCoWqgjyosLC/rXv9heMSPugnPOV+2Z+lv
CnDQG+vB5+lT3N7VK5WQBoJQouyDc0Y3P1RixZwKPKQzre9GCOPyvgboXlyX08dW
pfvyoeECgYEA0/Bi7rXiYdnKvY8z9F95rMdvgrU7HarxPQIY1nRcvlCVCVKpThRY
+BlAY2WChV0gElo6PtnkaBIcolyal/NjIc47dRlc+17V12u6vxegTqDPTV9uZ/C9
h0WxnD+CNkQDD/vGZ3S4Juo02tlr4VhwW6AmnKnlV2B0YDtW+HANxGQ=
-----END RSA PRIVATE KEY-----`),
			Pub: []byte(`ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCi7KAZaloUaM2zJjtdSkUZt4S1eDtYp0UjhDP+jceYY7BSvXCbCM5jLq0lKbXVrSWe3M8pg2/jH4yu6gDI8SMD5RgxIdhw5ZqgpwY85lxc/xpkVXZWAVOft6D24oNmEMJwRrSUHzTnswTNodmBlTQHz5N2aMXV17zRrCUAQZkrcqyNmoUQMD8WuX/I1VTUrLCD1vH0S49exPnRVVYGNrxajmcL9M5m3w0WevKZav4naiRXWkpxwPXrRMwqb+UvkYoggzPVmLyhhwWsJ1FXaWdyQlh39AzrCsDhwsPmU3//PCRWapQ9jkuJjWgSlnoFPaEfuMAKMbRlwVT4fGaFAXxr ekontsevoy@turing`),
		},
		{
			Priv: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEArBZaj7vxmgO/Gj2ph1TRjs8Ty9gQJ95Y2uhuK+ww+gNyWo07
BnKp8rUUGIp2IiqHOKDhDgkP8QUI1DhBScdfZscUPVnuQF+HCdAdNePz9cLqoSEk
IdQZ3slW53KbTIWA2PmEHleff/JeLZFgPrg0n9MUMlkId+YxqUYFjoxBtCXDtBHV
MwABKjG5+4ObyCp5vMx496aROOfg5na7WZ3sObjBBO+ve1MEqGE0qZhfaibCFP3a
ihztY6tv4nW+RuS3CLK5xjmnqBPE4EFgr4Vqg2g44LnPqD/RUADu8vvE5lGweG3l
tj6MPYomCAPzhcxJ40hUtWK9N8IO+e9rXb3bmQIDAQABAoIBAQCklzTC6MVpw0+S
b4unzm4oItMSUnMRTs65gTludRda6NUE2rOrtRvq8VppJnVatEZk2Sqn2+8NXP1W
zP9U64XJrXskOtFvbG6h6hUmKAJ7+pOizSnb2RttRDEEaU8z3zSfUfcVdkUtgMim
2LavBkv+2Uol5ZX954N0HW7PKkLlYu7JYpMG3NtAq+GWMSzDx2Piv3/QgMyAEzQ0
qd/T89LVuo7ax7axCD3PmSkD9vGO+xShNjz2oqPAQW5mBUk5uzYiyJCp6gfcnoJv
PxCrZ5lrrDlPeZQ2Yk/WB6SZ2VgJM1SMTTje5Dhsdg4X0yP14zbgmgVgho1XMyfV
WiVW0YABAoGBAOFYvy6GvY8p6uclFsOQZrd75Ug3WOXgAIS2DRjuRfrD779VE+fG
kKGTYa50Iol1SxftT6Ucc6bYDOg0Gh+cksPMcMBL5XjU45NAteRoCcULCKWCI4+4
w9HqjqKW1yuoP/bp74QqMrZk4s6e/1JgNQMmrR9nGPxB7JL1plfhqhgBAoGBAMN+
9IkEMvMiYeqfhgPff5JpZ1KqZMwwJ1tfceP4Tcda0wr+RWOcGH4vT+nOtE7nTexZ
WQ3qRvHtXQryT4BQ9UQVmlUVGOpOrH5QyfsWvf5PiKIdRS52vVvZ1E5c/AklD42Y
x0T3THDHOINKVU7MgwICmJYYz6O1/09E6ZY8zYOZAoGBANqyWWCbDY7KXJoVKaGE
G9vIlv1eEZ2OppIlaFKgtDOpQpzKwbW3xJe6xBsdxILo3YcMHbadBTSQCv6zygKR
3vG9EFPflIWO/onjTGOuAIVFrw+JXF/YLdskq2bpw0swT1ufL39xwKO5B1EFh773
dZtoRq3qTZpLlIAPfW9ep8gBAoGAcrnCT9ZDACQhSksrnoI+n3FzzTNpy9pGfnzY
nWxOWLuYNk9Z8UbdqM+jGhbQAa4EMLuOY3glAjzF6XKh7S+Vf8sdsuiaooZg/A/1
OID0JpYOHPUIcGgGYCzJRuOSlNtG8VXDO1nVZinDpGiu/3tNNpTHbu5IjE518dMD
McOk56ECgYEAufiEubSp9zwG+l49MMO1kdiXchSu97BtuS42Pp/zVknas+9KsmIC
iRYwgp9bmImLOQt4k+EYPYHJpzLAMECAUg/u5BwZxATtaocOhloxbFAudEtznJTw
1saWPyr9hkL1OKjiI/TLEAGx57cCiU0kyOS8KdzGtIR22LtHldHlgb8=
-----END RSA PRIVATE KEY-----`),
			Pub: []byte(`ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsFlqPu/GaA78aPamHVNGOzxPL2BAn3lja6G4r7DD6A3JajTsGcqnytRQYinYiKoc4oOEOCQ/xBQjUOEFJx19mxxQ9We5AX4cJ0B014/P1wuqhISQh1BneyVbncptMhYDY+YQeV59/8l4tkWA+uDSf0xQyWQh35jGpRgWOjEG0JcO0EdUzAAEqMbn7g5vIKnm8zHj3ppE45+DmdrtZnew5uMEE7697UwSoYTSpmF9qJsIU/dqKHO1jq2/idb5G5LcIsrnGOaeoE8TgQWCvhWqDaDjguc+oP9FQAO7y+8TmUbB4beW2Pow9iiYIA/OFzEnjSFS1Yr03wg7572tdvduZ ekontsevoy@turing`),
		},
	}
)
