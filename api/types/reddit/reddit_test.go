package reddit_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types/reddit"
)

var _ = Describe("Response", func() {
	Context("Unmarshalling JSON", func() {
		It("should correctly unmarshal a UserResponse", func() {
			jsonData := `{"type": "user", "id": "u1", "username": "testuser"}`
			var resp reddit.Response
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.User).NotTo(BeNil())
			Expect(resp.User.ID).To(Equal("u1"))
			Expect(resp.User.Username).To(Equal("testuser"))
			Expect(resp.Post).To(BeNil())
			Expect(resp.Comment).To(BeNil())
			Expect(resp.Community).To(BeNil())
		})

		It("should correctly unmarshal a PostResponse", func() {
			jsonData := `{"type": "post", "id": "p1", "title": "Test Post"}`
			var resp reddit.Response
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Post).NotTo(BeNil())
			Expect(resp.Post.ID).To(Equal("p1"))
			Expect(resp.Post.Title).To(Equal("Test Post"))
			Expect(resp.User).To(BeNil())
			Expect(resp.Comment).To(BeNil())
			Expect(resp.Community).To(BeNil())
		})

		It("should correctly unmarshal a CommentResponse", func() {
			jsonData := `{"type": "comment", "id": "c1", "body": "Test Comment"}`
			var resp reddit.Response
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Comment).NotTo(BeNil())
			Expect(resp.Comment.ID).To(Equal("c1"))
			Expect(resp.Comment.Body).To(Equal("Test Comment"))
			Expect(resp.User).To(BeNil())
			Expect(resp.Post).To(BeNil())
			Expect(resp.Community).To(BeNil())
		})

		It("should correctly unmarshal a CommunityResponse", func() {
			jsonData := `{"type": "community", "id": "co1", "name": "Test Community"}`
			var resp reddit.Response
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Community).NotTo(BeNil())
			Expect(resp.Community.ID).To(Equal("co1"))
			Expect(resp.Community.Name).To(Equal("Test Community"))
			Expect(resp.User).To(BeNil())
			Expect(resp.Post).To(BeNil())
			Expect(resp.Comment).To(BeNil())
		})

		It("should return an error for an unknown type", func() {
			jsonData := `{"type": "unknown", "id": "u1"}`
			var resp reddit.Response
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown Reddit response type: unknown"))
		})

		It("should return an error for invalid JSON", func() {
			jsonData := `{"type": "user", "id": "u1"`
			var resp reddit.Response
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Marshalling JSON", func() {
		It("should correctly marshal a UserResponse", func() {
			resp := reddit.Response{
				TypeSwitch: &reddit.TypeSwitch{Type: reddit.UserResponse},
				User:       &reddit.User{ID: "u1", Username: "testuser", DataType: "user"},
			}
			jsonData, err := json.Marshal(&resp)
			Expect(err).NotTo(HaveOccurred())
			expectedJSON := `{"id":"u1","url":"","username":"testuser","userIcon":"","postKarma":0,"commentKarma":0,"description":"","over18":false,"createdAt":"0001-01-01T00:00:00Z","scrapedAt":"0001-01-01T00:00:00Z","dataType":"user"}`
			Expect(jsonData).To(MatchJSON(expectedJSON))
		})

		It("should correctly marshal a PostResponse", func() {
			resp := reddit.Response{
				TypeSwitch: &reddit.TypeSwitch{Type: reddit.PostResponse},
				Post:       &reddit.Post{ID: "p1", Title: "Test Post", DataType: "post"},
			}
			jsonData, err := json.Marshal(&resp)
			Expect(err).NotTo(HaveOccurred())
			expectedJSON := `{"id":"p1","parsedId":"","url":"","username":"","title":"Test Post","communityName":"","parsedCommunityName":"","body":"","html":null,"numberOfComments":0,"upVotes":0,"isVideo":false,"isAd":false,"over18":false,"createdAt":"0001-01-01T00:00:00Z","scrapedAt":"0001-01-01T00:00:00Z","dataType":"post"}`
			Expect(jsonData).To(MatchJSON(expectedJSON))
		})

		It("should correctly marshal a CommentResponse", func() {
			now := time.Now().UTC()
			resp := reddit.Response{
				TypeSwitch: &reddit.TypeSwitch{Type: reddit.CommentResponse},
				Comment:    &reddit.Comment{ID: "c1", Body: "Test Comment", CreatedAt: now, ScrapedAt: now, DataType: "comment"},
			}
			jsonData, err := json.Marshal(&resp)
			Expect(err).NotTo(HaveOccurred())

			expectedComment := &reddit.Comment{ID: "c1", Body: "Test Comment", CreatedAt: now, ScrapedAt: now, DataType: "comment"}
			expectedJSON, _ := json.Marshal(expectedComment)
			Expect(jsonData).To(MatchJSON(expectedJSON))
		})

		It("should correctly marshal a CommunityResponse", func() {
			now := time.Now().UTC()
			resp := reddit.Response{
				TypeSwitch: &reddit.TypeSwitch{Type: reddit.CommunityResponse},
				Community:  &reddit.Community{ID: "co1", Name: "Test Community", CreatedAt: now, ScrapedAt: now, DataType: "community"},
			}
			jsonData, err := json.Marshal(&resp)
			Expect(err).NotTo(HaveOccurred())

			expectedCommunity := &reddit.Community{ID: "co1", Name: "Test Community", CreatedAt: now, ScrapedAt: now, DataType: "community"}
			expectedJSON, _ := json.Marshal(expectedCommunity)
			Expect(jsonData).To(MatchJSON(expectedJSON))
		})

		It("should return an error for an unknown type", func() {
			resp := reddit.Response{
				TypeSwitch: &reddit.TypeSwitch{Type: "unknown"},
			}
			_, err := json.Marshal(&resp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown Reddit response type: unknown"))
		})

		It("should marshal to null if TypeSwitch is nil", func() {
			resp := reddit.Response{}
			jsonData, err := json.Marshal(&resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(jsonData)).To(Equal("null"))
		})
	})
})
