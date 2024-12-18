/*
Copyright 2022 Gravitational, Inc.

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

// Package ppk_test provides tests for the ppk package
package ppk_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
)

func TestConvertToPPK(t *testing.T) {
	tests := []struct {
		desc   string
		priv   []byte
		pub    []byte
		output []byte
	}{
		{
			desc: "RSA key 1",
			priv: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA3U4OOAi+F1Ct1n8HZIs1P39CWB0mKLvshouuklenZug27SuI
14rjE+hOTNHYz/Pkvk5mmKuIdegMCe8FHAF6chygcEC9BDkowLO+2+f3sazGsu4A
9H4pDuUkuIM9MwmZV7A4TJ19rRAgha+6JKKR5KeEosfiLvAtOu2Pjqz8ZrOrUUqQ
1AJ71SkWMPTJFksTNmgaH7a0SgJ4vVYMlYIAeyoAgqn6Qvu5Kez5ROfeKD4zys/+
iFenrgbJrC38GNe2rxtb8/gfy03023FlPAQjGd1VLjxm8jhcJFqgM+uHTGRckgjv
d+VIkCbvTwpPWvvZxQcRtk073P9G8xpiNz2qbwIDAQABAoIBAQCFv37obqA0BxaI
5AzbvyZXUdoO1s8RH0I7rn+7Ai6yCvXnMMBrRA0pIuTvmIOoaoZ8XXW0HzdByxQ7
jLFR07Lk9Fgif328566xh/B5hyAzyW/tA9qf6P93eRVQTkDWb561WFMuOqCRz4VY
RnQBYB88SeHnX1Zbd9xeGOUCHZoNlrilVpgjscGcFNxyDP72qvI79z1vV+R6dhaf
YI2v1D6aqx9qM988ytOokNi79wYvSUxqitz3IOD5nBd9ZNBC0fDeVmHqqbHSvLrr
LouF7PiUuVA2LaWfVCy5dVtLkS16qbsfqzUA4B8Eg/oF0vPpJ7QMVxKI5j2//ScL
lQ9h6gUBAoGBAOQ0t9gGuHKOMcp3H9C2fzNVbbWTubJoUyzFGyx+U2aJ4byRbxS3
5d9cVu1GpS2ZgW6izCmxTG61Q0qQd4iT8e5cnFRU1Q3aK29TTK5hptthknXwKkVN
vUtlYKRM3TPYeTJ3WMQCY/Lzm2uVhT2ZGkpu0NaA5qiWllyPm7HlyQA/AoGBAPhC
KzioaPlqzwNKtHCsDSyeXsxU1aJCuMCIcgOB1yzmaaeL95CwMouMgouFyQ/CtLtO
pQEjymGzVynwC15s1vh1nCOWlQCx6Cjs9ko9bmecqziyyWg94gn82yLU7gClQH6v
+ezQ1n7/pb1DO/8dytO3+BZKSQH9lobzravGTcnRAoGAL8nKZfaiUXrtelSP2Qke
ggV1v/x7epzWLh3ontylYmelWfOqq1AHV0ri+TU+CdqHfD+jOWfjdZuHx+mQ3oz8
sMm8Avzw0MHLLrjm6e2RH4fDP+dXMsQgy9Ui88UU3XKLjsHnWMSXYZ0aAuGA0XFq
TAQAv6qmos9GFYQNOqe/+8kCgYEAv88H69eae5J9bTKr5R3Zc+7MmZy2Do70hbUm
OfV4lbVUTmJDHWQ1OUKPnlL4fJfX4Zwquo23kPLqVnmjnwoCsabUw15Vs1rBX9Vt
mQCLq7wNQlpIaKTfXw4hFXFkjdUf1oIKXGEiSK8mk+s9kKepDRlnsXklnUcbpRri
xQQLF/ECgYAmKBSQtPuyA9d3dAZj96HhYZzDjD2EtAhSUyx31vgqr8C7mmShQXLh
kFap4eAldBxySXp/5af7H1Xf4BIfbbc1prMM1vIRFTN6l6rbircak7bb9a/dgWmX
iukFsFq0G0Y2zt9oHOB7pKV/Kff4o1WQ0hcCBD6pZGhbsVxXBi4Oaw==
-----END RSA PRIVATE KEY-----
`),
			output: []byte(`PuTTY-User-Key-File-3: ssh-rsa
Encryption: none
Comment: teleport-generated-ppk
Public-Lines: 6
AAAAB3NzaC1yc2EAAAADAQABAAABAQDdTg44CL4XUK3WfwdkizU/f0JYHSYou+yG
i66SV6dm6DbtK4jXiuMT6E5M0djP8+S+TmaYq4h16AwJ7wUcAXpyHKBwQL0EOSjA
s77b5/exrMay7gD0fikO5SS4gz0zCZlXsDhMnX2tECCFr7okopHkp4Six+Iu8C06
7Y+OrPxms6tRSpDUAnvVKRYw9MkWSxM2aBoftrRKAni9VgyVggB7KgCCqfpC+7kp
7PlE594oPjPKz/6IV6euBsmsLfwY17avG1vz+B/LTfTbcWU8BCMZ3VUuPGbyOFwk
WqAz64dMZFySCO935UiQJu9PCk9a+9nFBxG2TTvc/0bzGmI3Papv
Private-Lines: 14
AAABAQCFv37obqA0BxaI5AzbvyZXUdoO1s8RH0I7rn+7Ai6yCvXnMMBrRA0pIuTv
mIOoaoZ8XXW0HzdByxQ7jLFR07Lk9Fgif328566xh/B5hyAzyW/tA9qf6P93eRVQ
TkDWb561WFMuOqCRz4VYRnQBYB88SeHnX1Zbd9xeGOUCHZoNlrilVpgjscGcFNxy
DP72qvI79z1vV+R6dhafYI2v1D6aqx9qM988ytOokNi79wYvSUxqitz3IOD5nBd9
ZNBC0fDeVmHqqbHSvLrrLouF7PiUuVA2LaWfVCy5dVtLkS16qbsfqzUA4B8Eg/oF
0vPpJ7QMVxKI5j2//ScLlQ9h6gUBAAAAgQD4Qis4qGj5as8DSrRwrA0snl7MVNWi
QrjAiHIDgdcs5mmni/eQsDKLjIKLhckPwrS7TqUBI8phs1cp8AtebNb4dZwjlpUA
sego7PZKPW5nnKs4ssloPeIJ/Nsi1O4ApUB+r/ns0NZ+/6W9Qzv/HcrTt/gWSkkB
/ZaG862rxk3J0QAAAIEA5DS32Aa4co4xyncf0LZ/M1VttZO5smhTLMUbLH5TZonh
vJFvFLfl31xW7UalLZmBbqLMKbFMbrVDSpB3iJPx7lycVFTVDdorb1NMrmGm22GS
dfAqRU29S2VgpEzdM9h5MndYxAJj8vOba5WFPZkaSm7Q1oDmqJaWXI+bseXJAD8A
AACBAM6/w3llPMNA/ZRm8wIXXssPgAZCN79zYtVu6n4KMqBzi7qj1er4gzsLZpKS
hpfdO/mDPhA3eFwU3XjYCKlHiJJYk53mc5sWwvbsfibAZSZAII/V4xWvRUUPE9EX
INDa/8cd4YSy3PiZnUTNLVb2SmRFhnlB8ZBk3CyGEvcHskir
Private-MAC: 2697903ac84b70273afc7adaa4e3ebb14536cdaf69654d40e3d46a5ba997ffb0
`),
		},
		{
			desc: "RSA key 2",
			priv: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAve2um90K1SkpJD1vcjm2zUYUh5ZU7q1cmO7F0J/6MCEcq3vH
fDPpPZ4uGLB9jPKzs6FYWhwFNW2oAsDvWSrwwxy5gl1dAdqp1wIm86gafShR0se5
rSdhWKP40H2lHOysRC5Jr8cvVLgflvZ4PDMqr/63BKwwkT1vN2PuenYRAAIT77X9
O0fumGQPKIxRGn5OPKEt1LzQ0+e/QlWrqZwzWDx5jqG3jbxibdcR/mHS60XdvusL
UqxxWPjhVlDsKfvh2lt5sqsjulWW/GyNtlCfaTn2uu0nV8nbT2OvEO+oM/uyHos5
7aIyePcOzCVM4dug6xJinqYTaVUsskKjPGUV6QIDAQABAoIBAQCBfis9k6DOIukt
D0IL5DOxk2Vt6F5x+PsYPjva+SfwZrMQbC1fjlkpLM8LAFIpplRFVe1SSqZ2fhQ+
BGNsLS3IKa6FprhCCl8f/BSoreWZjcLz7j63QxFJCUscg33u0aLGPbT5xtmLbpoD
KHpjuRMSuZz475mRfQx1/IldL2B52sIAD6XRTgFoRG+mLu2iNVvuE0RVbASiyOUs
lVwrGRI+5GuH8G6fDCJqpYzcm/S8VXmQc2jrbo/gQ76MkFxULqEMzadjN+XMXms7
pGZLX6Hatubn1kmhl8l6+1GYLf1HVmWXoL+hgWwbfIn6WV9y/xpnoeoJfWcFLJli
yABDx/mBAoGBAPhw3thyEP+5jdH2n1vz4X76yUbNJXaJGXozdoNfFKNOrYjFCLnD
CzHJEQmDJoFCtF6TwgvFb90HNvtNLkbC81yotQ8rfDzNTlixUhycaSsCJBqw0loU
wXoYQZiXpbfqT9Y7x7pwMxzRtkQYvyaowc7qF1xwJHhyCjDx38jAGnZxAoGBAMO1
DXUpca09h+FujJkziyJStYq0YKqsuKXW7CuAq2iY70lzhv+SIPErqcYIWwi8JNv9
EwBlEmSltFyGtxpeIVl6MJTil3vQ6eOSBCwt/E1YKvZoLv6mDf52Lc/wKtlecRPG
Q7G2C1ioTD9lDiYysUDmkpfitiatFwEj+y606wL5AoGBAMlQJLM9Ets1D19QuWb4
YwPS0aBGgZHgnD1yUBk5xW5jRajrCBwGmR6Zb+3GUUAyvhdZIccKEJAI1Zuiudnr
BOpTZovJT92w+0hRP1khwPJxxLHAEGOgJ/r4hsbQMx+phVHylPBVFIXIxSm+5726
x3kUJSPpVxQmTG3GwPBaAddxAoGALq+4QCTc22j8S0jl/X4QSOXWLPqOvOhrPBSj
TlVpjpA9NRZ8M+eWODIkU/uWS+UmHdyndcamtp/ZAOGaOI4QApplkH7liEH0Kbeh
izCFKaZIyXNdEp5mZDepAhvW/PfMnd0ENRaqakHrvovK7k3VfxgCDH2m2l8cR8df
mmrKTXECgYEA300gTnT46pMU1Wr1Zq4vGauWzk3U4J9HUu3vNy+sg4EEZ9CoiNTw
0a3f8u8gNQjB30koGW/5jYex3fUcnjTPqEGaiiGjI4oxMhquzqkVQ8FwnBAXJgT8
nQVO8MZw8iFeSap0ILum8t60sp1/u9aCWJbjPtb/fhx0q7SLdjFEw8s=
-----END RSA PRIVATE KEY-----
`),
			output: []byte(`PuTTY-User-Key-File-3: ssh-rsa
Encryption: none
Comment: teleport-generated-ppk
Public-Lines: 6
AAAAB3NzaC1yc2EAAAADAQABAAABAQC97a6b3QrVKSkkPW9yObbNRhSHllTurVyY
7sXQn/owIRyre8d8M+k9ni4YsH2M8rOzoVhaHAU1bagCwO9ZKvDDHLmCXV0B2qnX
AibzqBp9KFHSx7mtJ2FYo/jQfaUc7KxELkmvxy9UuB+W9ng8Myqv/rcErDCRPW83
Y+56dhEAAhPvtf07R+6YZA8ojFEafk48oS3UvNDT579CVaupnDNYPHmOobeNvGJt
1xH+YdLrRd2+6wtSrHFY+OFWUOwp++HaW3myqyO6VZb8bI22UJ9pOfa67SdXydtP
Y68Q76gz+7IeizntojJ49w7MJUzh26DrEmKephNpVSyyQqM8ZRXp
Private-Lines: 14
AAABAQCBfis9k6DOIuktD0IL5DOxk2Vt6F5x+PsYPjva+SfwZrMQbC1fjlkpLM8L
AFIpplRFVe1SSqZ2fhQ+BGNsLS3IKa6FprhCCl8f/BSoreWZjcLz7j63QxFJCUsc
g33u0aLGPbT5xtmLbpoDKHpjuRMSuZz475mRfQx1/IldL2B52sIAD6XRTgFoRG+m
Lu2iNVvuE0RVbASiyOUslVwrGRI+5GuH8G6fDCJqpYzcm/S8VXmQc2jrbo/gQ76M
kFxULqEMzadjN+XMXms7pGZLX6Hatubn1kmhl8l6+1GYLf1HVmWXoL+hgWwbfIn6
WV9y/xpnoeoJfWcFLJliyABDx/mBAAAAgQDDtQ11KXGtPYfhboyZM4siUrWKtGCq
rLil1uwrgKtomO9Jc4b/kiDxK6nGCFsIvCTb/RMAZRJkpbRchrcaXiFZejCU4pd7
0OnjkgQsLfxNWCr2aC7+pg3+di3P8CrZXnETxkOxtgtYqEw/ZQ4mMrFA5pKX4rYm
rRcBI/sutOsC+QAAAIEA+HDe2HIQ/7mN0fafW/PhfvrJRs0ldokZejN2g18Uo06t
iMUIucMLMckRCYMmgUK0XpPCC8Vv3Qc2+00uRsLzXKi1Dyt8PM1OWLFSHJxpKwIk
GrDSWhTBehhBmJelt+pP1jvHunAzHNG2RBi/JqjBzuoXXHAkeHIKMPHfyMAadnEA
AACAE820IDiCymxsVqgmBSNJttApBaSl3ljTzWWeJQR7ksIm9kBvy30j1682v0yq
RyPuY1EmQ3DJ3LqXbFq4qK12R/tALasyYyDYsJTt1xh+peFv23OSF8kDlG4MOdUp
3WPivAMSPR0QR192Emb0caXEkyAhvQLHKGoi8/TgbfMG6Gc=
Private-MAC: b5ede95d052e23815c8e8d816c758fb16370fc3178e1613fee61ec158900fd64
`),
		},
		{
			desc: "RSA key 3",
			priv: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAz5J/f572H95c9DDZLrXT0kmjytznkvntSOjxmJM44fL8DQz2
NINFi4awTNYD1eIIzaO4LLw+uXFWKD2P9LgtJ/Cxdb9LRi1OZ5Qrw/jj173zf/g+
wpItjoakgAzerHxKAPj3DB8iHFfPq+3MfdY36SZHT0GOU7QIhnYULKWWuVfexx25
VtgdGsmL9jwfAftzh00aCIej9zi2eSfGYfcIeRlSh9wvoYldrZbRvLTeMbW+YznW
kH4W9taCGofrq/t8tN0beh9B7z2hMGxOLLnsxu3gQc2KIUqU5l1myL0rVncvSwZw
ppQudZYtRyzmLOOm9PEvJHWvgu6KQBj5F24xrwIDAQABAoIBAQC0BgOMJMqjkxAd
POxvhYUjoXhr7bDuGNKB5H38bNrto/aUPwSdQKilPPhUe1yyOCqYZwDJ06222aP2
nIXooX+QX0EZtQHM6GhSjwByI78/kl/IQf30dCEMtpue7wqEn/ry4vooSiwkVsgm
/cPX811kWS2JgHq2/7JRI8GVgzu4m/wLtOVUIUiSG/zNZWx/ThEvvE/528z5MZG5
zGuQobHH+zfGYqk9IABcpNMH+4S353oPXAej2bCsQU6x+alM5z0fi+PuWIWtaDIb
e/Va9WN2fghXF5lxu/+sCv8QkoPotbRfh0nLO0nTt4MUIFR0X/mVXbVWn+5SBhWC
YUgcjychAoGBAOtLKKqkYzuOIyB2E3b7dPJ1XuzHOXj0Co5DoVNNs8TyEggoQPuj
cTLUQaIN+M+MyNmtLi4GaF1dXRrJg7qZoJ681Vz0P+w+pso1UTQcja5G8iOwiKAD
MIkyH9t9iW8yDN+J0dEzTqAgOPIDxkwDWuvwvsBleJ2EAV6qdecjLpIRAoGBAOHW
0NGHYe4GCbt/gA5UVUYXehx9mckcLwyZJJThjTZXYr1kglRYa4de5YRMk9oPCHUu
ODKqxL8CTcKyIijj1fJGDVcqTPFXlS4UZ31RLMvVnDaMID7V2zx+wxJ9onwhj798
1k3fVahH2vXOFH9AogeHKDNyD1RdwDNOhBy95Me/AoGALV+bAf0dXbi1MWdTrZgk
HzVfDs4EWTzGZFTKYWQUjKAZthT9IwmLpL+lwHhtSKjfeoqY4ys9KPP+JlJB4tQJ
U1Ma2ggH46jZRRkvBZuT/s2TmCpMzn6O94YA+rSkshq2vMy491yrhtlv4cu0i6gB
+om8XyGyNr3j/btlbSMtseECgYB66UL1Bk2SEc8yMI4tPlC6uQRIhUMxZRlmLeLu
9GK6dIzUruMPrJ+5KTiY7GR7hTsBK4qCaNZzbnmLwQ8+WeGS3fVcvzTpFNWoIorA
dXF/7l36ggD6scGEByl74syP6mQlv3eTIj2oPJM6vFIDf9WvayvB9A3LyMpWIiFc
0yy0WQKBgQDCPCUvQhiOJyQ63n3pjFl5/YOtadl9KUD/CmdyUkCt69QoFgG0wTAV
qalC9sysLQ1QI8A8GHNoNPjqMi7SWvzSgYN9TDRjS5GRlH13EALzP7AhWJWDoLYU
9DXNAEQrPMtX4Lzre7FmrYqEYqwdcac+vyXVgDA7ti1LhDhj8mm3Sg==
-----END RSA PRIVATE KEY-----
`),
			output: []byte(`PuTTY-User-Key-File-3: ssh-rsa
Encryption: none
Comment: teleport-generated-ppk
Public-Lines: 6
AAAAB3NzaC1yc2EAAAADAQABAAABAQDPkn9/nvYf3lz0MNkutdPSSaPK3OeS+e1I
6PGYkzjh8vwNDPY0g0WLhrBM1gPV4gjNo7gsvD65cVYoPY/0uC0n8LF1v0tGLU5n
lCvD+OPXvfN/+D7Cki2OhqSADN6sfEoA+PcMHyIcV8+r7cx91jfpJkdPQY5TtAiG
dhQspZa5V97HHblW2B0ayYv2PB8B+3OHTRoIh6P3OLZ5J8Zh9wh5GVKH3C+hiV2t
ltG8tN4xtb5jOdaQfhb21oIah+ur+3y03Rt6H0HvPaEwbE4suezG7eBBzYohSpTm
XWbIvStWdy9LBnCmlC51li1HLOYs46b08S8kda+C7opAGPkXbjGv
Private-Lines: 14
AAABAQC0BgOMJMqjkxAdPOxvhYUjoXhr7bDuGNKB5H38bNrto/aUPwSdQKilPPhU
e1yyOCqYZwDJ06222aP2nIXooX+QX0EZtQHM6GhSjwByI78/kl/IQf30dCEMtpue
7wqEn/ry4vooSiwkVsgm/cPX811kWS2JgHq2/7JRI8GVgzu4m/wLtOVUIUiSG/zN
ZWx/ThEvvE/528z5MZG5zGuQobHH+zfGYqk9IABcpNMH+4S353oPXAej2bCsQU6x
+alM5z0fi+PuWIWtaDIbe/Va9WN2fghXF5lxu/+sCv8QkoPotbRfh0nLO0nTt4MU
IFR0X/mVXbVWn+5SBhWCYUgcjychAAAAgQDh1tDRh2HuBgm7f4AOVFVGF3ocfZnJ
HC8MmSSU4Y02V2K9ZIJUWGuHXuWETJPaDwh1LjgyqsS/Ak3CsiIo49XyRg1XKkzx
V5UuFGd9USzL1Zw2jCA+1ds8fsMSfaJ8IY+/fNZN31WoR9r1zhR/QKIHhygzcg9U
XcAzToQcveTHvwAAAIEA60soqqRjO44jIHYTdvt08nVe7Mc5ePQKjkOhU02zxPIS
CChA+6NxMtRBog34z4zI2a0uLgZoXV1dGsmDupmgnrzVXPQ/7D6myjVRNByNrkby
I7CIoAMwiTIf232JbzIM34nR0TNOoCA48gPGTANa6/C+wGV4nYQBXqp15yMukhEA
AACAJ2iqIoXMYc0w3sXBQJ2BJyRYFBlZ0Czrz7xZEaBXrK5BcZjCARnmAp2Hfuvx
i0lz0PHAz9f6hpjZuLEGLO7f3kGMcyEquYd89FHvP1yLxggYiXGKNDYSDZRK8Yy7
MipqcnT4j5zDuFi744aO5fIchKp02z+ttGVt/i5zuGNh+do=
Private-MAC: a9b12c6450e46fd7abbaaff5841f8a64f9597c7b2b59bd69d6fd3ceee0ca61ea
`),
		},
		{
			desc: "ed25519 key",
			priv: []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtz
c2gtZWQyNTUxOQAAACBj4UfPX3B2yLRkt8ABGWiQGME1oY7N7K8yMTECt4HTvgAA
AIjWpv6D1qb+gwAAAAtzc2gtZWQyNTUxOQAAACBj4UfPX3B2yLRkt8ABGWiQGME1
oY7N7K8yMTECt4HTvgAAAEBW11q/rO8oWVkJGVV0md/Q7MQMkoisjyqKdk/aFQpl
U2PhR89fcHbItGS3wAEZaJAYwTWhjs3srzIxMQK3gdO+AAAAAAECAwQF
-----END OPENSSH PRIVATE KEY-----`),
			output: []byte(`PuTTY-User-Key-File-3: ssh-ed25519
Encryption: none
Comment: teleport-generated-ppk
Public-Lines: 2
AAAAC3NzaC1lZDI1NTE5AAAAIGPhR89fcHbItGS3wAEZaJAYwTWhjs3srzIxMQK3
gdO+
Private-Lines: 1
AAAAIFbXWr+s7yhZWQkZVXSZ39DsxAySiKyPKop2T9oVCmVT
Private-MAC: 69e26c50e92d520bef9a19913b54b9585bcadbc3ba8eb01eadf95c9c4e5e5f4e
`),
		},
		{
			desc: "ecdsa key",
			priv: []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAaAAAABNl
Y2RzYS1zaGEyLW5pc3RwMjU2AAAACG5pc3RwMjU2AAAAQQR46dTQpQmnDizIvtPH
rQ9bOtCD73Jt98YCundWBx2wZxvtAi3OT15Ku/R65Qu2E/6psMdYeADta7DgKtmy
HT3AAAAAoKbt/b2m7f29AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAy
NTYAAABBBHjp1NClCacOLMi+08etD1s60IPvcm33xgK6d1YHHbBnG+0CLc5PXkq7
9HrlC7YT/qmwx1h4AO1rsOAq2bIdPcAAAAAhAObTnBS3qFRxz272PVnDJ37EVyH2
Ryfdptn0Kw5TyRq7AAAAAAECAwQFBgc=
-----END OPENSSH PRIVATE KEY-----`),
			output: []byte(`PuTTY-User-Key-File-3: ecdsa-sha2-nistp256
Encryption: none
Comment: teleport-generated-ppk
Public-Lines: 3
AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBHjp1NClCacO
LMi+08etD1s60IPvcm33xgK6d1YHHbBnG+0CLc5PXkq79HrlC7YT/qmwx1h4AO1r
sOAq2bIdPcA=
Private-Lines: 1
AAAAIQDm05wUt6hUcc9u9j1Zwyd+xFch9kcn3abZ9CsOU8kauw==
Private-MAC: 6e788dafd452d27c17d062add28113d59d03a20898ea89046e3809fe38832861
`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			priv, err := keys.ParsePrivateKey(tc.priv)
			require.NoError(t, err)

			output, err := priv.PPKFile()
			require.NoError(t, err)
			require.Equal(t, output, tc.output)
		})
	}
}
