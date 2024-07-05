package types

type Id int64

type ResourceType string

type ResourceMapping struct {
	ResourceId   Id           `json:"resource_id"`
	ResourceType ResourceType `json:"resource_type"`
}
