/*
 * Model of lib/events/filesessions as it is on this branch: the bug.
 *
 * CompleteUpload opens <sid>.tar with O_TRUNC BEFORE taking the flock, so it
 * empties the recording out from under the async uploader that is streaming it.
 * flock is advisory and per-inode, and by the time the lock is even requested
 * the damage is done.
 *
 * Both processes work on the same <sid>.tar because service.go passes one
 * uploadsDir as the uploader's ScanDir and the Handler's Directory.
 */

// Both CompleteUpload and startUpload lock <sid>.tar itself.
enum tLockTarget { LOCK_DATA_PATH }

event eOpen: machine;              // open(path, O_RDWR) -- binds to the current inode
event eOpenResult;
event eOpenTrunc: machine;         // open(path, O_RDWR|O_CREATE|O_TRUNC) -- empties the LIVE file
event eOpenTruncResult;
event eTryLock: (requester: machine, target: tLockTarget);
event eTryLockResult: bool;
event eUnlock: machine;
event eReadPart: (requester: machine, part: int);   // CombineParts, into the live file
event eReadPartResult: bool;
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
  var partsPresent: bool;

  start state Init {
    entry (seed: seq[int]) {
      pathInode = 1;
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

    // The truncate that is the bug: it empties the live recording, and it takes
    // no lock because O_TRUNC is a side effect of open(2).
    on eOpenTrunc do (requester: machine) {
      fdInode[requester] = pathInode;
      contentOf[pathInode] = default(seq[int]);
      send requester, eOpenTruncResult;
    }

    on eTryLock do (req: (requester: machine, target: tLockTarget)) {
      var inode: int;
      inode = pathInode;
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

    // Read a source part and copy it into the caller's descriptor, which for
    // the completer is the live recording.
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
 *   GetOpenFileFunc()(uploadPath, O_RDWR|O_CREATE|O_TRUNC, 0o600)  // truncates FIRST
 *   FSTryWriteLock(uploadPath)                // ...and only then asks for the lock
 *   CombineParts(ctx, f, ...)                 // writes into the live recording
 *   os.RemoveAll(uploadRootPath)
 *
 * The truncate is not covered by the lock, and losing the lock race afterwards
 * does not undo it.
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
      send fs, eOpenTrunc, this;
      goto Opening;
    }
  }

  state Opening {
    on eOpenTruncResult do { goto Locking; }
  }

  state Locking {
    entry { send fs, eTryLock, (requester = this, target = LOCK_DATA_PATH); }
    on eTryLockResult do (ok: bool) {
      if (ok) { nextOrFinish(); goto Combining; }
      attemptsLeft = attemptsLeft - 1;
      // Gives up and returns an error -- but the recording is already empty.
      if (attemptsLeft <= 0) { goto Done; }
      goto Locking;
    }
  }

  state Combining {
    on eReadPartResult do (ok: bool) {
      if (!ok) {
        send fs, eUnlock, this;
        goto Done;
      }
      nextPart = nextPart + 1;
      nextOrFinish();
    }
    on eRemovePartsResult do { send fs, eUnlock, this; goto Done; }
  }

  fun nextOrFinish() {
    if (nextPart >= sizeof(parts)) {
      send fs, eRemoveParts, this;
    } else {
      send fs, eReadPart, (requester = this, part = parts[nextPart]);
    }
  }

  state Done { ignore eOpenTruncResult, eTryLockResult, eReadPartResult, eRemovePartsResult; }
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
