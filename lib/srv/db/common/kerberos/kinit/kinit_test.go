/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package kinit

import (
	"context"
	_ "embed"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata/kinit.cache
var cacheData []byte
var badCacheData = []byte("bad cache data to write to file")

type staticCache struct {
	t    *testing.T
	pass bool
}

type badCache struct {
	t *testing.T
}

func (b *badCache) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cachePath := args[len(args)-1]
	require.NotEmpty(b.t, cachePath)
	err := os.WriteFile(cachePath, badCacheData, 0664)
	require.NoError(b.t, err)

	return exec.Command("echo")
}

func (s *staticCache) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cachePath := args[len(args)-1]
	require.NotEmpty(s.t, cachePath)
	err := os.WriteFile(cachePath, cacheData, 0664)
	require.NoError(s.t, err)

	if s.pass {
		return exec.Command("echo")
	}
	cmd := exec.Command("")
	cmd.Err = errors.New("bad command")
	return cmd

}

type testCertGetter struct {
	pass bool
}

func (t *testCertGetter) GetCertificateBytes(context.Context) (*WindowsCAAndKeyPair, error) {
	if t.pass {
		return &WindowsCAAndKeyPair{}, nil
	}
	return nil, errors.New("could not get cert bytes")

}

type testCase struct {
	name           string
	initializer    *PKInit
	expectErr      require.ErrorAssertionFunc
	expectCacheNil require.ValueAssertionFunc
	expectConfNil  require.ValueAssertionFunc
}

func step(t *testing.T, name string, cg CommandGenerator, c *testCertGetter, expectErr require.ErrorAssertionFunc, expectNil require.ValueAssertionFunc) *testCase {
	t.Helper()

	dir := t.TempDir()
	var err error
	dir, err = os.MkdirTemp(dir, "krb5_cache")
	require.NoError(t, err)

	return &testCase{
		name: name,
		initializer: New(NewCommandLineInitializer(
			CommandConfig{
				User:        "alice",
				Realm:       "example.com",
				KDCHost:     "host.example.com",
				AdminServer: "host.example.com",
				DataDir:     dir,
				Command:     cg,
				CertGetter:  c,
			})),
		expectErr:      expectErr,
		expectCacheNil: expectNil,
		expectConfNil:  expectNil,
	}
}

func TestNewWithCommandLineProvider(t *testing.T) {

	for _, tt := range []struct {
		name         string
		cg           CommandGenerator
		c            *testCertGetter
		expectErr    require.ErrorAssertionFunc
		expectReturn require.ValueAssertionFunc
	}{

		{
			name:         "CommandSuccessCase",
			cg:           &staticCache{t: t, pass: true},
			c:            &testCertGetter{pass: true},
			expectErr:    require.NoError,
			expectReturn: require.NotNil},
		{
			name:         "CertificateFailureCase",
			cg:           &staticCache{t: t, pass: true},
			c:            &testCertGetter{pass: false},
			expectErr:    require.Error,
			expectReturn: require.Nil},
		{
			name:         "CommandFailureCase",
			cg:           &staticCache{t: t, pass: false},
			c:            &testCertGetter{pass: true},
			expectErr:    require.Error,
			expectReturn: require.Nil},
		{
			name:         "BadCacheData",
			cg:           &badCache{t: t},
			c:            &testCertGetter{pass: true},
			expectErr:    require.Error,
			expectReturn: require.Nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tc := step(t, tt.name, tt.cg, tt.c, tt.expectErr, tt.expectReturn)
			c, conf, err := tc.initializer.UseOrCreateCredentialsCache(context.Background())
			tc.expectErr(t, err)
			tc.expectCacheNil(t, c)
			tc.expectConfNil(t, conf)
		})
	}

}

const (
	expectedConfString = `[libdefaults]
 default_realm = example.com
 rdns = false


[realms]
 example.com = {
  kdc = host.example.com
  admin_server = host.example.com
  pkinit_eku_checking = kpServerAuth
  pkinit_kdc_hostname = instance.host.example.com
 }`
)

func TestKRBConfString(t *testing.T) {
	cli := NewCommandLineInitializer(
		CommandConfig{
			User:        "alice",
			Realm:       "example.com",
			KDCHost:     "instance.host.example.com",
			AdminServer: "host.example.com",
			Command:     &staticCache{t: t, pass: true},
			CertGetter:  &testCertGetter{pass: true},
			DataDir:     t.TempDir(),
		})

	tmp := t.TempDir()

	f := filepath.Join(tmp, "krb.conf")
	err := cli.WriteKRB5Config(f)
	require.NoError(t, err)

	data, err := os.ReadFile(f)
	require.NoError(t, err)

	require.Equal(t, expectedConfString, string(data))

	// Ensure that the returned configuration matches the information from the
	// generated file. PKINIT options are not available on the go-krb5 config.
	_, conf, err := cli.UseOrCreateCredentials(context.Background())
	require.NoError(t, err)
	require.Len(t, conf.Realms, 1)
	require.Equal(t, "example.com", conf.Realms[0].Realm)
	require.ElementsMatch(t, []string{"host.example.com"}, conf.Realms[0].AdminServer)
	require.ElementsMatch(t, []string{"host.example.com:88"}, conf.Realms[0].KDC)
	require.Equal(t, "example.com", conf.LibDefaults.DefaultRealm)
	require.False(t, conf.LibDefaults.RDNS)
}
