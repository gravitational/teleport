package pluginsv1

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

const (
	// maxDynamoDBSize is the maximum size of a DynamoDB item.
	// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Constraints.html#limits-items
	maxDynamoDBItemSize = 400 * 1024 // 400KB

	trimMsg = "Error message is too large and is trimmed. See Auth log for more details.\n"
)

func trimToMaxSize(in *types.PluginV1, maxSize int) *types.PluginV1 {
	if in.Size() < maxSize {
		return in
	}

	out := utils.CloneProtoMsg(in)
	fieldCount := 0
	if out.Status.ErrorMessage != "" {
		fieldCount++
	}
	if out.Status.LastRawError != "" {
		fieldCount++
	}

	// Zero value for trimmable fields.
	out.Status.ErrorMessage = ""
	out.Status.LastRawError = ""

	if fieldCount == 0 || out.Size() > maxSize {
		// Let the plugin service deal with the outsized
		// plugin as the final out size is going to be
		// larger despite trimming the status fields.
		return in
	}
	// Ensures that the out size always remains below the bounds of maxSize.
	maxOutSize := maxSize - maxSize/10
	maxSizePerField := maxOutSize / fieldCount

	// [trimStr] func is copied from the /api/types/events/events.go.
	trimStr := func(s string, n int) string {
		// Starting at 2 to leave room for quotes at the begging and end.
		charCount := 2
		for i := range len(s) {
			// Make sure we always have room to add an escape character if necessary.
			if charCount+1 > n {
				return s[:i]
			}
			r := rune(s[i])
			if r == '"' || r == '\\' {
				charCount++
			}
			charCount++
		}
		return s
	}

	out.Status.ErrorMessage = trimStr(in.GetStatus().GetErrorMessage(), maxSizePerField)
	msgToTrim := in.GetStatus().GetLastRawError()
	if msgToTrim != "" {
		msgToTrim = trimMsg + msgToTrim
	}
	out.Status.LastRawError = trimStr(msgToTrim, maxSizePerField)

	out.SetStatus(&out.Status)
	return out
}
