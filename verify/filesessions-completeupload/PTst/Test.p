/*
 * A completed <sid>.tar on disk, the async uploader streaming it, and a
 * re-completion for the same session arriving concurrently.
 */
machine Driver {
  start state Init {
    entry {
      var fs: machine;
      var parts: seq[int];
      parts += (0, 1);
      parts += (1, 2);
      announce eExpectedRecording, parts;

      fs = new FileSystem(parts);
      new Uploader(fs);
      new Completer((fs = fs, parts = parts));
    }
  }
}

test tcNoTornUpload [main = Driver]:
  assert NoTornUpload in { FileSystem, Completer, Uploader, Driver };
