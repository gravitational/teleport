/*
 * Model of lib/events/filesessions as it is on this branch.
 *
 * The bug: CompleteUpload used to open <sid>.tar with O_TRUNC BEFORE taking the
 * flock, truncating the recording out from under the async uploader that was
 * streaming it. flock is advisory and per-inode, so it did not stop the
 * truncate.
 *
 * The fix: CompleteUpload assembles into a temp file and publishes with
 * os.Rename. The uploader's already-open descriptor keeps pointing at the old
 * inode, so it always streams a complete recording.
 *
 * Both processes work on the same <sid>.tar because service.go passes one
 * uploadsDir as the uploader's ScanDir and the Handler's Directory.
 */

// CompleteUpload locks <sid>.tar.lock (filestream.go); startUpload locks
// <sid>.tar (fileasync.go). They do not exclude each other, which is safe only
// because the recording is never modified in place.
enum tLockTarget { LOCK_DATA_PATH, LOCK_SIDECAR }

event eOpen: machine;              // open(path, O_RDWR) -- binds to the current inode
event eOpenResult;
event eOpenTemp: machine;          // open(tmpPath, O_CREATE|O_TRUNC) -- a fresh inode
event eOpenTempResult;
event eTryLock: (requester: machine, target: tLockTarget);
event eTryLockResult: bool;
event eUnlock: machine;
event eReadPart: (requester: machine, part: int);   // CombineParts, into the temp file
event eReadPartResult: bool;
event ePublish: machine;           // os.Rename(tmpPath, uploadPath)
event ePublishResult;
event eReadAll: machine;           // the uploader streaming its descriptor
event eReadAllResult: seq[int];
event eRemoveParts: machine;       // os.RemoveAll(uploadRootPath)
event eRemovePartsResult;

event eExpectedRecording: seq[int];
event eUploaderStreamed: seq[int];

/*
 * Contents live PER INODE and descriptors bind at open time. That is the whole
 * point: os.Rename swaps the inode behind the path, and a reader holding the
 * old one is unaffected.
 */
machine FileSystem {
  var contentOf: map[int, seq[int]];
  var fdInode: map[machine, int];
  var holderOfInode: map[int, machine];
  var lockedInodeOf: map[machine, int];
  var pathInode: int;
  var sidecarInode: int;
  var nextInode: int;
  var partsPresent: bool;

  start state Init {
    entry (seed: seq[int]) {
      nextInode = 2;
      pathInode = 1;
      sidecarInode = 2;
      contentOf[pathInode] = seed;
      partsPresent = true;
      goto Serving;
    }
  }

  state Serving {
    on eOpen do (requester: machine) {
      fdInode[requester] = pathInode;
      send requester, eOpenResult;
    }

    on eOpenTemp do (requester: machine) {
      nextInode = nextInode + 1;
      contentOf[nextInode] = default(seq[int]);
      fdInode[requester] = nextInode;
      send requester, eOpenTempResult;
    }

    on eTryLock do (req: (requester: machine, target: tLockTarget)) {
      var inode: int;
      if (req.target == LOCK_SIDECAR) { inode = sidecarInode; } else { inode = pathInode; }
      if (inode in holderOfInode) {
        send req.requester, eTryLockResult, false;
        return;
      }
      holderOfInode[inode] = req.requester;
      lockedInodeOf[req.requester] = inode;
      send req.requester, eTryLockResult, true;
    }

    on eUnlock do (requester: machine) {
      if (requester in lockedInodeOf) {
        holderOfInode -= (lockedInodeOf[requester]);
        lockedInodeOf -= (requester);
      }
    }

    // Read a source part and copy it into the caller's temp file.
    on eReadPart do (req: (requester: machine, part: int)) {
      var buf: seq[int];
      if (!partsPresent) {
        send req.requester, eReadPartResult, false;
        return;
      }
      buf = contentOf[fdInode[req.requester]];
      buf += (sizeof(buf), req.part);
      contentOf[fdInode[req.requester]] = buf;
      send req.requester, eReadPartResult, true;
    }

    // The path now resolves to the temp inode. Descriptors open elsewhere keep
    // the inode they were bound to.
    on ePublish do (requester: machine) {
      pathInode = fdInode[requester];
      send requester, ePublishResult;
    }

    on eReadAll do (requester: machine) {
      send requester, eReadAllResult, contentOf[fdInode[requester]];
    }

    on eRemoveParts do (requester: machine) {
      partsPresent = false;
      send requester, eRemovePartsResult;
    }
  }
}

/*
 * Handler.CompleteUpload (filestream.go):
 *   FSTryWriteLock(uploadPath + lockExt)      // sidecar, before touching anything
 *   CombineParts(ctx, tmpFile, ...)           // into a temp file
 *   os.Rename(tmpPath, uploadPath)            // publish in one step
 *   os.RemoveAll(uploadRootPath)
 */
machine Completer {
  var fs: machine;
  var parts: seq[int];
  var attemptsLeft: int;
  var nextPart: int;

  start state Init {
    entry (cfg: (fs: machine, parts: seq[int])) {
      fs = cfg.fs;
      parts = cfg.parts;
      attemptsLeft = 4;
      nextPart = 0;
      goto Locking;
    }
  }

  state Locking {
    entry { send fs, eTryLock, (requester = this, target = LOCK_SIDECAR); }
    on eTryLockResult do (ok: bool) {
      if (ok) { send fs, eOpenTemp, this; goto Combining; }
      attemptsLeft = attemptsLeft - 1;
      if (attemptsLeft <= 0) { goto Done; }
      goto Locking;
    }
  }

  state Combining {
    on eOpenTempResult do { nextOrPublish(); }
    on eReadPartResult do (ok: bool) {
      if (!ok) {
        // The parts are gone; return an error having touched nothing.
        send fs, eUnlock, this;
        goto Done;
      }
      nextPart = nextPart + 1;
      nextOrPublish();
    }
    on ePublishResult do { send fs, eRemoveParts, this; }
    on eRemovePartsResult do { send fs, eUnlock, this; goto Done; }
  }

  fun nextOrPublish() {
    if (nextPart >= sizeof(parts)) {
      send fs, ePublish, this;
    } else {
      send fs, eReadPart, (requester = this, part = parts[nextPart]);
    }
  }

  state Done { ignore eTryLockResult, eOpenTempResult, eReadPartResult, ePublishResult, eRemovePartsResult; }
}

/*
 * Uploader.startUpload (fileasync.go):
 *   os.OpenFile(sessionFilePath, os.O_RDWR, 0)
 *   utils.FSTryWriteLock(sessionFilePath)     // the tar itself, not the sidecar
 *   ... stream it to the audit backend
 *
 * The open happens before the lock here too, but harmlessly: there is no
 * O_TRUNC. The read window is the upload duration, which for a remote backend
 * is long -- that is the window the bug exploited.
 */
machine Uploader {
  var fs: machine;
  var attemptsLeft: int;

  start state Init {
    entry (f: machine) {
      fs = f;
      attemptsLeft = 4;
      send fs, eOpen, this;
      goto Opening;
    }
  }

  state Opening {
    on eOpenResult do { goto Locking; }
  }

  state Locking {
    entry { send fs, eTryLock, (requester = this, target = LOCK_DATA_PATH); }
    on eTryLockResult do (ok: bool) {
      if (ok) { send fs, eReadAll, this; goto Reading; }
      attemptsLeft = attemptsLeft - 1;
      if (attemptsLeft <= 0) { goto Done; }
      goto Locking;
    }
  }

  state Reading {
    on eReadAllResult do (contents: seq[int]) {
      announce eUploaderStreamed, contents;
      send fs, eUnlock, this;
      goto Done;
    }
  }

  state Done { ignore eOpenResult, eTryLockResult, eReadAllResult; }
}
