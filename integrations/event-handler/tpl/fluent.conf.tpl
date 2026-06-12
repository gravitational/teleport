<source>
    @type http
    port 8888

    <transport tls>
        client_cert_auth true
        ca_path "/keys/{{.CaCertFileName}}"
        cert_path "/keys/{{.ServerCertFileName}}"
        private_key_path "/keys/{{.ServerKeyFileName}}"
        private_key_passphrase "{{.Pwd}}"
    </transport>

    <parse>
      @type json
      json_parser oj

      # This time format is used by Go marshaller
      time_type string
      time_format %Y-%m-%dT%H:%M:%S
    </parse>

    # If the number of events is high, fluentd will start failing the ingestion
    # with the following error message: buffer space has too many data errors.
    # The following configuration prevents data loss in case of a restart and
    # overcomes the limitations of the default fluentd buffer configuration.
    # This configuration is optional.
    # See https://docs.fluentd.org/configuration/buffer-section for more details.
    <buffer>
      @type file
      flush_thread_count 8
      flush_interval 1s
      chunk_limit_size 10M
      queue_limit_length 16
      retry_max_interval 30
      retry_forever true
    </buffer>
</source>

<match test.log>
  @type stdout
</match>

<match session.*.log>
  @type stdout
</match>
