package models

// Tour represents a full virtual tour
type Tour struct {
	ID                   string  `json:"id" gorm:"primaryKey;size:50;column:id"`
	Name                 string  `json:"name" gorm:"column:name"`
	UserID               string  `json:"user_id" gorm:"column:user_id;size:50;index"`     // Owner of the tour
	PropertyID           *string `json:"property_id,omitempty" gorm:"column:property_id"` // nullable - only if from main frontend
	BackgroundAudioURL   *string `json:"background_audio_url,omitempty" gorm:"column:background_audio_url"`
	IsPublished          bool    `json:"is_published" gorm:"column:is_published"`
	IsFeaturedOnHomepage bool    `json:"is_featured_on_homepage" gorm:"column:is_featured_on_homepage;default:false"`
	AutoplayEnabled      bool    `json:"autoplay_enabled" gorm:"column:autoplay_enabled"`  // nullable
	IsPaid               bool    `json:"is_paid" gorm:"column:is_paid;default:false"`      // Whether payment was made
	PaymentID            *string `json:"payment_id,omitempty" gorm:"column:payment_id"`    // Reference to payment
	Source               string  `json:"source" gorm:"column:source;default:'standalone'"` // 'main_app' or 'standalone'

	DefaultFOV        float64 `json:"default_fov" gorm:"default:75;column:default_fov"`
	DefaultYawSpeed   float64 `json:"default_yaw_speed" gorm:"default:0.01;column:default_yaw_speed"`
	DefaultPitchSpeed float64 `json:"default_pitch_speed" gorm:"default:0.0;column:default_pitch_speed"`

	// Auto-change settings
	AutoChangeEnabled      bool   `json:"auto_change_enabled" gorm:"column:auto_change_enabled;default:false"`
	AutoChangeInterval     int    `json:"auto_change_interval" gorm:"column:auto_change_interval;default:5000"` // milliseconds
	AutoChangeMode         string `json:"auto_change_mode" gorm:"column:auto_change_mode;default:'sequential'"` // sequential, random
	AutoPauseOnInteraction bool   `json:"auto_pause_on_interaction" gorm:"column:auto_pause_on_interaction;default:true"`
	AutoRestartDelay       int    `json:"auto_restart_delay" gorm:"column:auto_restart_delay;default:30000"` // milliseconds to wait before restarting after interaction

	// Thumbnail URL from first scene (not stored in DB, populated at runtime)
	ThumbnailURL *string `json:"thumbnail_url,omitempty" gorm:"-"`

	TourScenes []TourScene `json:"tour_scenes" gorm:"foreignKey:TourID"`

	BaseModel
}

// TourWithProperty represents a tour with property information included
type TourWithProperty struct {
	Tour
	PropertyName *string `json:"property_name,omitempty"`
}

// TourScene maps which scenes are part of a tour and their sequence
type TourScene struct {
	ID            string `json:"id" gorm:"primaryKey;size:50;column:id"`
	TourID        string `json:"tour_id" gorm:"index;column:tour_id"`
	SceneID       string `json:"scene_id" gorm:"index;column:scene_id"`
	SequenceOrder int    `json:"sequence_order" gorm:"column:sequence_order"` // scene order in this tour

	BaseModel
}

// Scene represents a single scene in a tour
type Scene struct {
	ID                 string  `json:"id" gorm:"primaryKey;size:50;column:id"`
	Name               string  `json:"name" gorm:"column:name"`
	Type               string  `json:"type" gorm:"column:type"` // image, video, 360
	SrcOriginalURL     *string `json:"src_original_url,omitempty" gorm:"column:src_original_url"`
	CubemapManifestURL *string `json:"cubemap_manifest_url,omitempty" gorm:"column:cubemap_manifest_url"`
	TilesManifest      *string `json:"tiles_manifest,omitempty" gorm:"type:jsonb;column:tiles_manifest"` // Store tiles manifest JSON in DB
	Yaw                float64 `json:"yaw" gorm:"column:yaw"`
	Pitch              float64 `json:"pitch" gorm:"column:pitch"`
	FOV                float64 `json:"fov" gorm:"column:fov"`
	Order              int     `json:"order" gorm:"column:scene_order"`
	Priority           int     `json:"priority" gorm:"column:priority;default:0"`
	TourID             string  `json:"tour_id" gorm:"index;column:tour_id"`

	// Auto-change settings for this specific scene
	Duration           *int    `json:"duration,omitempty" gorm:"column:duration"`                          // Override tour's auto_change_interval for this scene (milliseconds)
	TransitionType     string  `json:"transition_type" gorm:"column:transition_type;default:'fade'"`       // fade, slide, zoom, cut
	TransitionDuration int     `json:"transition_duration" gorm:"column:transition_duration;default:1000"` // milliseconds
	AutoRotate         bool    `json:"auto_rotate" gorm:"column:auto_rotate;default:false"`                // Auto-rotate scene while displayed
	AutoRotateSpeed    float64 `json:"auto_rotate_speed" gorm:"column:auto_rotate_speed;default:0.5"`      // degrees per frame

	Hotspots []Hotspot `json:"hotspots" gorm:"foreignKey:SceneID"`
	Overlays []Overlay `json:"overlays" gorm:"foreignKey:SceneID"`

	BaseModel
}

// Hotspot represents clickable points in a scene
type Hotspot struct {
	ID                  string  `json:"id" gorm:"primaryKey;size:50;column:id"`
	TourID              string  `json:"tour_id" gorm:"index;column:tour_id"`
	SceneID             string  `json:"scene_id" gorm:"index;column:scene_id"`
	TargetSceneID       string  `json:"target_scene_id" gorm:"index;column:target_scene_id"`
	Kind                string  `json:"kind" gorm:"column:kind"` // navigation, info, image
	Yaw                 float64 `json:"yaw" gorm:"column:yaw"`
	Pitch               float64 `json:"pitch" gorm:"column:pitch"`
	TransitionDirection string  `json:"transition_direction" gorm:"column:transition_direction"` // forward, backward, left, right, up, down

	Payload string `json:"payload,omitempty" gorm:"type:jsonb;column:payload"` // supports multiple targets + rotation points

	BaseModel
}

// Overlay represents extra content in a scene
type Overlay struct {
	ID      string  `json:"id" gorm:"primaryKey;size:50;column:id"`
	TourID  string  `json:"tour_id" gorm:"index;column:tour_id"`
	SceneID string  `json:"scene_id" gorm:"index;column:scene_id"`
	Kind    string  `json:"kind" gorm:"column:kind"` // text, image, video
	Yaw     float64 `json:"yaw" gorm:"column:yaw"`
	Pitch   float64 `json:"pitch" gorm:"column:pitch"`

	Payload string `json:"payload,omitempty" gorm:"type:jsonb;column:payload"`

	BaseModel
}

// PlayTour represents a selective tour with custom camera movements
type PlayTour struct {
	ID     string `json:"id" gorm:"primaryKey;size:50;column:id"`
	TourID string `json:"tour_id" gorm:"index;column:tour_id"`
	Name   string `json:"name" gorm:"column:name"`
	UserID string `json:"user_id" gorm:"column:user_id;size:50;index"`

	PlayTourScenes []PlayTourScene `json:"play_tour_scenes" gorm:"foreignKey:PlayTourID"`

	BaseModel
}

// PlayTourScene defines a scene in a PlayTour with its camera movement path
type PlayTourScene struct {
	ID            string `json:"id" gorm:"primaryKey;size:50;column:id"`
	PlayTourID    string `json:"play_tour_id" gorm:"index;column:play_tour_id"`
	SceneID       string `json:"scene_id" gorm:"index;column:scene_id"`
	SequenceOrder int    `json:"sequence_order" gorm:"column:sequence_order"`

	// Start camera position
	StartYaw   float64 `json:"start_yaw" gorm:"column:start_yaw"`
	StartPitch float64 `json:"start_pitch" gorm:"column:start_pitch"`
	StartFOV   float64 `json:"start_fov" gorm:"column:start_fov"`

	// End camera position
	EndYaw   float64 `json:"end_yaw" gorm:"column:end_yaw"`
	EndPitch float64 `json:"end_pitch" gorm:"column:end_pitch"`
	EndFOV   float64 `json:"end_fov" gorm:"column:end_fov"`

	MoveDuration        int     `json:"move_duration" gorm:"column:move_duration;default:5000"` // milliseconds
	WaitDuration        int     `json:"wait_duration" gorm:"column:wait_duration;default:2000"` // milliseconds
	TransitionDirection string  `json:"transition_direction" gorm:"column:transition_direction;default:forward"`
	Title               *string `json:"title,omitempty" gorm:"column:title"`
	Description         *string `json:"description,omitempty" gorm:"column:description"`

	BaseModel
}
