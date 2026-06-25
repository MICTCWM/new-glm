package stream_notice

import "math/rand"

// retryMessages is a pool of friendly, varied English messages sent to users
// while a retry is in flight. Each entry aims to reassure the user that work
// is actively happening behind the scenes instead of showing a bare "wait".
//
// Guidelines used when writing these:
//   - varied sentence openers (Let me..., Give me..., Bear with me..., etc.)
//   - polite, warm, slightly conversational tone
//   - no promises of specific durations
//   - no philosophical platitudes or excessive flattery
//   - each line is 15-40 words long
var retryMessages = []string{
	"Let me gather my thoughts and try a different approach for you, this may take a moment.",
	"Give me a second to recalibrate — I'm reworking the answer so it comes out right, please hang tight.",
	"Bear with me while I double-check a few things behind the scenes, almost there.",
	"Hang on a moment, I want to make sure I get this right for you on the next try.",
	"Just a sec — I'm reorganizing my response to be clearer and more accurate.",
	"Working on it from a fresh angle, give me a brief moment to put the pieces together.",
	"Hold tight, I'm refining my reasoning so the answer lands better the second time around.",
	"Allow me a moment to step back and reconsider the question more carefully.",
	"Let me rethink this one step at a time, your patience is genuinely appreciated.",
	"Give me a moment to shake off the hiccup and come back with something solid.",
	"Bear with me, I'm going to walk through the details again to avoid missing anything.",
	"Hang on, I'm re-reading what came before so my next attempt builds on it properly.",
	"Just a moment, I want to verify a couple of facts before answering again.",
	"Working on a cleaner take — sit tight, it's coming together nicely.",
	"Hold on while I tighten up the wording so the response reads more naturally.",
	"Allow me a second to regroup and tackle this from a better angle.",
	"Let me pause and reconstruct the answer with more care this time around.",
	"Give me a moment, I'm stitching together a more coherent reply for you.",
	"Bear with me while I tidy up the logic — almost ready to send again.",
	"Hang on, I want to make the next attempt sharper and more to the point.",
	"Just a sec, I'm adjusting my approach so the answer feels less scattered.",
	"Working on it — I'll be back with a more thoughtful response shortly.",
	"Hold tight while I double-check the tricky parts before trying once more.",
	"Allow me a brief pause to line up my thoughts and try again with confidence.",
	"Let me take another swing at this, please give me a moment to collect myself.",
	"Give me a second to smooth out the rough edges and come back with a better reply.",
	"Bear with me, I'm going over the question again to make sure I haven't missed the point.",
	"Hang on just a moment, I want to restructure the answer so it flows better.",
	"Just a sec — reorganizing things behind the scenes, thanks for waiting.",
	"Working on a fresh take, I'll have a cleaner answer ready in just a bit.",
	"Hold on while I reconsider which details actually matter here.",
	"Allow me a moment to recalibrate my reasoning and give it another go.",
	"Let me rework the response so it's actually helpful, hang in there with me.",
	"Give me a moment to re-examine my assumptions before answering again.",
	"Bear with me, I'm carefully rebuilding the answer from the ground up.",
	"Hang on, I want to make sure the next reply doesn't repeat the same stumble.",
	"Just a moment, I'm gathering the relevant pieces back together.",
	"Working on it — give me a beat to settle on the right framing for this.",
	"Hold tight, I'm rethinking how to explain this in a way that actually clicks.",
	"Allow me a second to breathe and approach the question with fresh eyes.",
	"Let me pull my thoughts together and try once more, almost there.",
	"Give me a moment to retrace my steps and catch where I went off track.",
	"Bear with me while I refine the wording so it doesn't feel rushed.",
	"Hang on, I'm re-evaluating the question to give you a more useful answer.",
	"Just a sec, I want to align my response more closely with what you're after.",
	"Working on a better version — sit tight, it's shaping up nicely.",
	"Hold on while I sort through the details and pick out what really matters.",
	"Allow me a moment to regroup, then I'll come back with a stronger attempt.",
	"Let me reframe the problem and try again, thanks for bearing with me.",
	"Give me a second to make sure I'm not missing anything obvious this time.",
	"Bear with me, I'm going to lay out the reasoning more carefully on the next pass.",
	"Hang on just a moment, I want to tighten the answer up before sending it again.",
	"Just a sec — I'm recomposing my thoughts so the reply lands more cleanly.",
	"Working on it, give me a moment to find a clearer path through this.",
	"Hold tight while I revisit the tricky bits and smooth them out.",
	"Allow me a brief moment to re-center and try the question from a new angle.",
	"Let me rethink the approach and come back with something that actually helps.",
	"Give me a moment to clean up my reasoning, I'll be right with you.",
	"Bear with me while I piece together a more polished response.",
	"Hang on, I'm taking a careful second pass so the answer holds up this time.",
}

// RandomRetryMessage returns a randomly chosen retry message, terminated
// with a newline so it can be streamed directly as a thinking delta.
func RandomRetryMessage() string {
	msg := retryMessages[rand.Intn(len(retryMessages))]
	return msg + "\n"
}
