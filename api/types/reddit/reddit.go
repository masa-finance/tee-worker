package reddit

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/masa-finance/tee-types/pkg/util"
)

type QueryType string

const (
	ScrapeUrls        QueryType = "scrapeurls"
	SearchPosts       QueryType = "searchposts"
	SearchUsers       QueryType = "searchusers"
	SearchCommunities QueryType = "searchcommunities"
)

var AllQueryTypes = util.NewSet(ScrapeUrls, SearchPosts, SearchUsers, SearchCommunities)

type SortType string

const (
	SortRelevance SortType = "relevance"
	SortHot       SortType = "hot"
	SortTop       SortType = "top"
	SortNew       SortType = "new"
	SortRising    SortType = "rising"
	SortComments  SortType = "comments"
)

var AllSortTypes = util.NewSet(
	SortRelevance,
	SortHot,
	SortTop,
	SortNew,
	SortRising,
	SortComments,
)

// StartURL represents a single start URL for the Apify Reddit scraper.
type StartURL struct {
	URL    string `json:"url"`
	Method string `json:"method"`
}

type ResponseType string

const (
	UserResponse      ResponseType = "user"
	PostResponse      ResponseType = "post"
	CommentResponse   ResponseType = "comment"
	CommunityResponse ResponseType = "community"
)

// User represents the data structure for a Reddit user from the Apify scraper.
type User struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	Username     string    `json:"username"`
	UserIcon     string    `json:"userIcon"`
	PostKarma    int       `json:"postKarma"`
	CommentKarma int       `json:"commentKarma"`
	Description  string    `json:"description"`
	Over18       bool      `json:"over18"`
	CreatedAt    time.Time `json:"createdAt"`
	ScrapedAt    time.Time `json:"scrapedAt"`
	DataType     string    `json:"dataType"`
}

// Post represents the data structure for a Reddit post from the Apify scraper.
type Post struct {
	ID                  string    `json:"id"`
	ParsedID            string    `json:"parsedId"`
	URL                 string    `json:"url"`
	Username            string    `json:"username"`
	Title               string    `json:"title"`
	CommunityName       string    `json:"communityName"`
	ParsedCommunityName string    `json:"parsedCommunityName"`
	Body                string    `json:"body"`
	HTML                *string   `json:"html"`
	NumberOfComments    int       `json:"numberOfComments"`
	UpVotes             int       `json:"upVotes"`
	IsVideo             bool      `json:"isVideo"`
	IsAd                bool      `json:"isAd"`
	Over18              bool      `json:"over18"`
	CreatedAt           time.Time `json:"createdAt"`
	ScrapedAt           time.Time `json:"scrapedAt"`
	DataType            string    `json:"dataType"`
}

// Comment represents the data structure for a Reddit comment from the Apify scraper.
type Comment struct {
	ID              string    `json:"id"`
	ParsedID        string    `json:"parsedId"`
	URL             string    `json:"url"`
	ParentID        string    `json:"parentId"`
	Username        string    `json:"username"`
	Category        string    `json:"category"`
	CommunityName   string    `json:"communityName"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"createdAt"`
	ScrapedAt       time.Time `json:"scrapedAt"`
	UpVotes         int       `json:"upVotes"`
	NumberOfReplies int       `json:"numberOfreplies"`
	HTML            string    `json:"html"`
	DataType        string    `json:"dataType"`
}

// Community represents the data structure for a Reddit community from the Apify scraper.
type Community struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Title           string    `json:"title"`
	HeaderImage     string    `json:"headerImage"`
	Description     string    `json:"description"`
	Over18          bool      `json:"over18"`
	CreatedAt       time.Time `json:"createdAt"`
	ScrapedAt       time.Time `json:"scrapedAt"`
	NumberOfMembers int       `json:"numberOfMembers"`
	URL             string    `json:"url"`
	DataType        string    `json:"dataType"`
}

type TypeSwitch struct {
	Type ResponseType `json:"dataType"`
}

type Response struct {
	TypeSwitch *TypeSwitch
	User       *User
	Post       *Post
	Comment    *Comment
	Community  *Community
}

func (t *Response) UnmarshalJSON(data []byte) error {
	t.TypeSwitch = &TypeSwitch{}
	if err := json.Unmarshal(data, &t.TypeSwitch); err != nil {
		return fmt.Errorf("failed to unmarshal reddit response type: %w", err)
	}

	switch t.TypeSwitch.Type {
	case UserResponse:
		t.User = &User{}
		if err := json.Unmarshal(data, t.User); err != nil {
			return fmt.Errorf("failed to unmarshal reddit user: %w", err)
		}
	case PostResponse:
		t.Post = &Post{}
		if err := json.Unmarshal(data, t.Post); err != nil {
			return fmt.Errorf("failed to unmarshal reddit post: %w", err)
		}
	case CommentResponse:
		t.Comment = &Comment{}
		if err := json.Unmarshal(data, t.Comment); err != nil {
			return fmt.Errorf("failed to unmarshal reddit comment: %w", err)
		}
	case CommunityResponse:
		t.Community = &Community{}
		if err := json.Unmarshal(data, t.Community); err != nil {
			return fmt.Errorf("failed to unmarshal reddit community: %w", err)
		}
	default:
		return fmt.Errorf("unknown Reddit response type during unmarshal: %s", t.TypeSwitch.Type)
	}
	return nil
}

// MarshalJSON implements the json.Marshaler interface for Response.
// It unwraps the inner struct (User, Post, Comment, or Community) and marshals it directly.
func (t *Response) MarshalJSON() ([]byte, error) {
	if t.TypeSwitch == nil {
		return []byte("null"), nil
	}

	switch t.TypeSwitch.Type {
	case UserResponse:
		return json.Marshal(t.User)
	case PostResponse:
		return json.Marshal(t.Post)
	case CommentResponse:
		return json.Marshal(t.Comment)
	case CommunityResponse:
		return json.Marshal(t.Community)
	default:
		return nil, fmt.Errorf("unknown Reddit response type during marshal: %s", t.TypeSwitch.Type)
	}
}
