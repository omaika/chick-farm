package core_test

import (
	"testing"

	"github.com/sting8k/piggery/internal/core"
)

// mail (admin) lists a participant's messages newest first with names and where each stands, pages
// with before, and acks nothing: the recipient still gets its mail.
func TestMailListsWithoutAcking(t *testing.T) {
	f := newFixture(t, nil)
	first := f.send(t, f.alice, core.SendArgs{To: "bob", Body: "one"})
	f.inbox(t, f.bob, batch(1))
	if _, err := f.e.Completion(ctx, f.bob, core.CompletionArgs{Batch: 1}); err != nil {
		t.Fatal(err)
	}
	f.send(t, f.bob, core.SendArgs{To: "alice", Body: "two", ReplyTo: first.ID})
	f.send(t, f.alice, core.SendArgs{To: "bob", Body: "three"})

	r, err := f.e.Mail(ctx, core.MailArgs{Participant: f.bob.ParticipantID})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range r.Messages {
		got = append(got, m.From+">"+m.To+":"+m.Body+":"+m.State)
	}
	if want := []string{"alice>bob:three:pending", "bob>alice:two:pending", "alice>bob:one:acked"}; !eq(got, want) || r.More {
		t.Fatalf("mail = %v more=%v, want %v", got, r.More, want)
	}
	if two := r.Messages[1]; two.ReplyTo != r.Messages[2].Seq || two.Team == "" {
		t.Fatalf("reply_to %d (one is #%d), team %q", two.ReplyTo, r.Messages[2].Seq, two.Team)
	}

	page, err := f.e.Mail(ctx, core.MailArgs{Limit: 1, Before: r.Messages[0].Seq})
	if err != nil || len(page.Messages) != 1 || page.Messages[0].Body != "two" || !page.More {
		t.Fatalf("page = %+v, %v", page, err)
	}

	if d := f.inbox(t, f.bob, nil); len(d) != 1 || d[0].Body != "three" {
		t.Fatalf("bob's inbox after mail = %+v", d)
	}
}
