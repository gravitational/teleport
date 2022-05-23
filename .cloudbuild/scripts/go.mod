module github.com/gravitational/teleport/.cloudbuild/scripts

go 1.16

require (
	cloud.google.com/go/secretmanager v1.4.0
	github.com/go-git/go-git/v5 v5.4.2
	github.com/gravitational/trace v1.1.15
	github.com/sirupsen/logrus v1.8.1
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b
	google.golang.org/genproto v0.0.0-20220519153652-3a47de7e79bd
)

require github.com/jonboulle/clockwork v0.2.2 // indirect
