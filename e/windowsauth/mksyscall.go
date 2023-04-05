package main

// This command is used to generate system call bodies using https://pkg.go.dev/golang.org/x/sys/windows/mkwinsyscall
// Every line starting with //sys is one generated function

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go crypto.go credential_provider.go auth.go credential_provider_credential.go
