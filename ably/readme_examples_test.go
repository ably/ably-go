// Generated by test_readme_examples. DO NOT EDIT
package ably_test

import "testing"
import "github.com/ably/ably-go/ably"
import "github.com/ably/ably-go/ably/ablytest"

/* README.md:15 */ import "context"

func TestReadmeExamples(t *testing.T) {
	t.Parallel()

	fmt := struct {
		Println func(a ...interface{}) (n int, err error)
		Printf  func(s string, a ...interface{}) (n int, err error)
	}{
		Println: func(a ...interface{}) (n int, err error) { return 0, nil },
		Printf:  func(s string, a ...interface{}) (n int, err error) { return 0, nil },
	}

	app := ablytest.MustSandbox(nil)
	defer safeclose(t, app)
	/* README.md:18 */ ctx := context.Background()
	/* README.md:22 */ client, err := ably.NewRealtime(app.Options(ably.WithClientID("clientID"))...)
	/* README.md:23 */ if err != nil {
		/* README.md:24 */ panic(err)
		/* README.md:25 */
	}
	/* README.md:27 */ channel := client.Channels.Get("test")
	/* README.md:33 */ unsubscribe, err := channel.SubscribeAll(ctx, func(msg *ably.Message) {
		/* README.md:34 */ fmt.Printf("Received message: name=%s data=%v\n", msg.Name, msg.Data)
		/* README.md:35 */
	})
	/* README.md:36 */ if err != nil {
		/* README.md:37 */ panic(err)
		/* README.md:38 */
	}
	/* README.md:42 */ unsubscribe()
	/* README.md:48 */ unsubscribe1, err := channel.Subscribe(ctx, "EventName1", func(msg *ably.Message) {
		/* README.md:49 */ fmt.Printf("Received message: name=%s data=%v\n", msg.Name, msg.Data)
		/* README.md:50 */
	})
	/* README.md:51 */ if err != nil {
		/* README.md:52 */ panic(err)
		/* README.md:53 */
	}
	/* README.md:55 */ unsubscribe2, err := channel.Subscribe(ctx, "EventName2", func(msg *ably.Message) {
		/* README.md:56 */ fmt.Printf("Received message: name=%s data=%v\n", msg.Name, msg.Data)
		/* README.md:57 */
	})
	/* README.md:58 */ if err != nil {
		/* README.md:59 */ panic(err)
		/* README.md:60 */
	}
	/* README.md:64 */ unsubscribe1()
	/* README.md:65 */ unsubscribe2()
	/* README.md:71 */ err = channel.Publish(ctx, "EventName1", "EventData1")
	/* README.md:72 */ if err != nil {
		/* README.md:73 */ panic(err)
		/* README.md:74 */
	}
	/* README.md:80 */ err = channel.Presence.Enter(ctx, "presence data")
	/* README.md:81 */ if err != nil {
		/* README.md:82 */ panic(err)
		/* README.md:83 */
	}
	/* README.md:89 */ err = channel.Presence.EnterClient(ctx, "clientID", "presence data")
	/* README.md:90 */ if err != nil {
		/* README.md:91 */ panic(err)
		/* README.md:92 */
	}
	/* README.md:98 */ // Update also has an UpdateClient variant.
	/* README.md:99 */
	err = channel.Presence.Update(ctx, "new presence data")
	/* README.md:100 */ if err != nil {
		/* README.md:101 */ panic(err)
		/* README.md:102 */
	}
	/* README.md:104 */ // Leave also has an LeaveClient variant.
	/* README.md:105 */
	err = channel.Presence.Leave(ctx, "last presence data")
	/* README.md:106 */ if err != nil {
		/* README.md:107 */ panic(err)
		/* README.md:108 */
	}
	/* README.md:114 */ clients, err := channel.Presence.Get(ctx)
	/* README.md:115 */ if err != nil {
		/* README.md:116 */ panic(err)
		/* README.md:117 */
	}
	/* README.md:119 */ for _, client := range clients {
		/* README.md:120 */ fmt.Println("Present client:", client)
		/* README.md:121 */
	}
	/* README.md:127 */ unsubscribe, err = channel.Presence.SubscribeAll(ctx, func(msg *ably.PresenceMessage) {
		/* README.md:128 */ fmt.Printf("Presence event: action=%v data=%v", msg.Action, msg.Data)
		/* README.md:129 */
	})
	/* README.md:130 */ if err != nil {
		/* README.md:131 */ panic(err)
		/* README.md:132 */
	}
	/* README.md:136 */ unsubscribe()
	/* README.md:142 */ unsubscribe, err = channel.Presence.Subscribe(ctx, ably.PresenceActionEnter, func(msg *ably.PresenceMessage) {
		/* README.md:143 */ fmt.Printf("Presence event: action=%v data=%v", msg.Action, msg.Data)
		/* README.md:144 */
	})
	/* README.md:145 */ if err != nil {
		/* README.md:146 */ panic(err)
		/* README.md:147 */
	}
	/* README.md:151 */ unsubscribe()
	/* README.md:161 */ {
		/* README.md:165 */ client, err := ably.NewREST(app.Options(ably.WithClientID("clientID"))...)
		/* README.md:166 */ if err != nil {
			/* README.md:167 */ panic(err)
			/* README.md:168 */
		}
		/* README.md:170 */ channel := client.Channels.Get("test")
		/* README.md:176 */ err = channel.Publish(ctx, "HelloEvent", "Hello!")
		/* README.md:177 */ if err != nil {
			/* README.md:178 */ panic(err)
			/* README.md:179 */
		}
		/* README.md:181 */ // You can also publish a batch of messages in a single request.
		/* README.md:182 */
		err = channel.PublishBatch(ctx, []*ably.Message{
			/* README.md:183 */ {Name: "HelloEvent", Data: "Hello!"},
			/* README.md:184 */ {Name: "ByeEvent", Data: "Bye!"},
			/* README.md:185 */})
		/* README.md:186 */ if err != nil {
			/* README.md:187 */ panic(err)
			/* README.md:188 */
		}
		/* README.md:194 */ page, err := channel.History(nil)
		/* README.md:195 */ for ; err == nil && page != nil; page, err = page.Next() {
			/* README.md:196 */ for _, message := range page.Messages() {
				/* README.md:197 */ fmt.Println(message)
				/* README.md:198 */
			}
			/* README.md:199 */
		}
		/* README.md:200 */ if err != nil {
			/* README.md:201 */ panic(err)
			/* README.md:202 */
		}
		/* README.md:208 */ page, err = channel.Presence.Get(nil)
		/* README.md:209 */ for ; err == nil && page != nil; page, err = page.Next() {
			/* README.md:210 */ for _, presence := range page.PresenceMessages() {
				/* README.md:211 */ fmt.Println(presence)
				/* README.md:212 */
			}
			/* README.md:213 */
		}
		/* README.md:214 */ if err != nil {
			/* README.md:215 */ panic(err)
			/* README.md:216 */
		}
		/* README.md:222 */ page, err = channel.Presence.History(nil)
		/* README.md:223 */ for ; err == nil && page != nil; page, err = page.Next() {
			/* README.md:224 */ for _, presence := range page.PresenceMessages() {
				/* README.md:225 */ fmt.Println(presence)
				/* README.md:226 */
			}
			/* README.md:227 */
		}
		/* README.md:228 */ if err != nil {
			/* README.md:229 */ panic(err)
			/* README.md:230 */
		}
		/* README.md:236 */ page, err = client.Stats(&ably.PaginateParams{})
		/* README.md:237 */ for ; err == nil && page != nil; page, err = page.Next() {
			/* README.md:238 */ for _, stat := range page.Stats() {
				/* README.md:239 */ fmt.Println(stat)
				/* README.md:240 */
			}
			/* README.md:241 */
		}
		/* README.md:242 */ if err != nil {
			/* README.md:243 */ panic(err)
			/* README.md:244 */
		}
		/* README.md:248 */
	}
}