/*
 * The uploader must never stream a torn recording to the audit backend.
 *
 * This is the bug, stated as a property. Before the fix, a concurrent
 * CompleteUpload truncated <sid>.tar mid-upload and the uploader streamed
 * whatever was left -- see TestCompleteUploadTearsConcurrentUpload.
 */
spec NoTornUpload observes eExpectedRecording, eUploaderStreamed {
  var expected: seq[int];

  start state Monitoring {
    on eExpectedRecording do (e: seq[int]) { expected = e; }

    on eUploaderStreamed do (got: seq[int]) {
      assert sizeof(got) == sizeof(expected),
        format("TORN UPLOAD: uploader streamed {0} of {1} parts", sizeof(got), sizeof(expected));
    }
  }
}
