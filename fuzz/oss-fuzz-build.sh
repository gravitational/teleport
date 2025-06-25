#!/bin/bash -eu

TELEPORT_PREFIX="github.com/gravitational/teleport"

prepare_teleport() {

  go get github.com/AdamKorcz/go-118-fuzz-build/testing
  go get -u all || true
  go mod tidy
  go get github.com/AdamKorcz/go-118-fuzz-build/testing

  # Fix /root/go/pkg/mod/github.com/aws/aws-sdk-go-v2/internal/ini@v1.3.0/fuzz.go:13:21:
  # not enough arguments in call to Parse
  rm -f /root/go/pkg/mod/github.com/aws/aws-sdk-go-v2/internal/ini@*/fuzz.go

}

prepare_teleport_api() {

  (cd api; go get github.com/AdamKorcz/go-118-fuzz-build/testing)

}

build_teleport_fuzzers() {

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/desktop/tdp \
    FuzzDecode fuzz_decode

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/services \
    FuzzParseRefs fuzz_parse_refs

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/cassandra/protocol \
    FuzzReadPacket fuzz_cassandra_read_packet

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/elasticsearch \
    FuzzGetQueryFromRequestBody fuzz_elasticsearch_query_from_request_body

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/elasticsearch \
    FuzzPathToMatcher fuzz_elasticsearch_path_to_matcher

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/redis \
    FuzzParseRedisAddress fuzz_parse_redis_address

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/sshutils/sftp \
    FuzzParseDestination fuzz_sshutil_parse_destination

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/sshutils/x11 \
    FuzzParseDisplay fuzz_parse_display

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/sshutils/x11 \
    FuzzReadAndRewriteXAuthPacket fuzz_read_xauth_packet

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/utils/aws \
    FuzzParseSigV4 fuzz_parse_sig_v4

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/utils/parse \
    FuzzNewExpression fuzz_new_expression

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/utils/parse \
    FuzzNewMatcher fuzz_new_matcher

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/utils \
    FuzzParseProxyJump fuzz_parse_proxy_jump

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/utils \
    FuzzReadYAML fuzz_read_yaml

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/client \
    FuzzParseProxyHost fuzz_parse_proxy_host

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/client \
    FuzzParseLabelSpec fuzz_parse_label_spec

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/client \
    FuzzParseSearchKeywords fuzz_parse_search_keywords

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/client \
    FuzzParsePortForwardSpec fuzz_parse_port_forward_spec

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/client \
    FuzzParseDynamicPortForwardSpec fuzz_parse_dynamic_port_forward_spec

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/regular \
    FuzzParseProxySubsys fuzz_parse_proxy_subsys

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/kube/proxy \
    FuzzParseResourcePath fuzz_parse_resource_path

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/mysql/protocol \
    FuzzParsePacket fuzz_parse_mysql_packet

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/mysql/protocol \
    FuzzFetchMySQLVersion fuzz_fetch_mysql_version

# compile_native_go_fuzzer $TELEPORT_PREFIX/lib/auth \
#   FuzzParseAndVerifyIID fuzz_parse_and_verify_iid

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/sqlserver/protocol \
    FuzzMSSQLLogin fuzz_mssql_login

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/sqlserver/protocol \
    FuzzMSSQLRPCClientPartialLength fuzz_mssql_rpc_client_partial_length

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/opensearch \
    FuzzPathToMatcher fuzz_opensearch_path_to_matcher

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/auth/webauthn \
    FuzzParseCredentialCreationResponseBody fuzz_parse_credential_creation_response_body

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/auth/webauthn \
    FuzzParseCredentialRequestResponseBody fuzz_parse_credential_request_response_body

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/auth/webauthncli \
    FuzzParseU2FRegistrationResponse fuzz_parse_u2f_registration_response

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/web \
    FuzzTdpMFACodecDecodeChallenge fuzz_tdp_mfa_codec_decode_challenge

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/web \
    FuzzTdpMFACodecDecodeResponse fuzz_tdp_mfa_codec_decode_response

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/web \
    FuzzHandlePlaybackAction fuzz_handle_playback_action

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/multiplexer \
    FuzzReadProxyLineV1 fuzz_read_proxy_linec_v1

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/multiplexer \
    FuzzReadProxyLineV2 fuzz_read_proxy_linec_v2

}

build_teleport_api_fuzzers() {

  cd api

  compile_native_go_fuzzer $TELEPORT_PREFIX/api/types \
    FuzzParseDuration fuzz_parse_duration

  compile_native_go_fuzzer $TELEPORT_PREFIX/api/utils/aws \
    FuzzParseRDSEndpoint fuzz_parse_rds_endpoint

  compile_native_go_fuzzer $TELEPORT_PREFIX/api/utils/aws \
    FuzzParseRedshiftEndpoint fuzz_parse_redshift_endpoint

  compile_native_go_fuzzer $TELEPORT_PREFIX/api/utils/aws \
    FuzzParseElastiCacheEndpoint fuzz_parse_elasti_cache_endpoint

  compile_native_go_fuzzer $TELEPORT_PREFIX/api/utils/aws \
    FuzzParseDynamoDBEndpoint fuzz_parse_dynamodb_endpoint

  compile_native_go_fuzzer $TELEPORT_PREFIX/api/utils/azure \
    FuzzParseDatabaseEndpoint fuzz_azure_parse_database_endpoint

  compile_native_go_fuzzer $TELEPORT_PREFIX/api/utils/azure \
    FuzzParseCacheForRedisEndpoint fuzz_azure_parse_cache_for_redis_endpoint

  compile_native_go_fuzzer $TELEPORT_PREFIX/api/utils/azure \
    FuzzNormalizeLocation fuzz_azure_normalize_location

  compile_native_go_fuzzer $TELEPORT_PREFIX/api/utils/azure \
    FuzzParseMSSQLEndpoint fuzz_azure_parse_mssql_endpoint

  cd -

}

compile() {

  prepare_teleport
  prepare_teleport_api

  build_teleport_fuzzers
  build_teleport_api_fuzzers

}

copy_corpora() {

  # generate corpus
  for fuzzer_path in fuzz/corpora/fuzz_*
  do
    fuzzer_name=$OUT/$(basename "$fuzzer_path")
    rm -f "$fuzzer_name"_seed_corpus.zip
    zip --junk-paths "$fuzzer_name"_seed_corpus.zip $fuzzer_path/*
  done

}

copy_corpora
compile
