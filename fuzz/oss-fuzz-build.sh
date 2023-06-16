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
    FuzzParserEvalBoolPredicate fuzz_parser_eval_bool_predicate

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/restrictedsession \
    FuzzParseIPSpec fuzz_parse_ip_spec

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/services \
    FuzzParseRefs fuzz_parse_refs

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/redis \
    FuzzParseRedisAddress fuzz_parse_redis_address

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/sshutils/x11 \
    FuzzParseDisplay fuzz_parse_display

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/utils/aws \
    FuzzParseSigV4 fuzz_parse_sig_v4

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/utils/parse \
    FuzzNewExpression fuzz_new_expression

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/utils/parse \
    FuzzNewMatcher fuzz_new_matcher

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/utils \
    FuzzParseProxyJump fuzz_parse_proxy_jump

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/utils \
    FuzzParseWebLinks fuzz_parse_web_links

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/utils \
    FuzzReadYAML fuzz_read_yaml

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/client \
    FuzzParseProxyHost fuzz_parse_proxy_host

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

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/client \
    FuzzParseLabelSpec fuzz_parse_label_spec

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/sqlserver/protocol \
    FuzzMSSQLLogin fuzz_mssql_login

# Disabled until we can update the mongoDB driver
#  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/srv/db/mongodb/protocol \
#   FuzzMongoRead fuzz_mongo_read

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/auth/webauthn \
    FuzzParseCredentialCreationResponseBody fuzz_parse_credential_creation_response_body

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/auth/webauthn \
    FuzzParseCredentialRequestResponseBody fuzz_parse_credential_request_response_body

  compile_native_go_fuzzer $TELEPORT_PREFIX/lib/web \
    FuzzTdpMFACodecDecode fuzz_tdp_mfa_codec_decode

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
