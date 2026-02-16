package tui

// Re-export types from internal/office for backward compatibility.
// This allows existing TUI code to continue using these types without
// changing imports throughout the codebase.

import "github.com/binaryphile/sofdevsim-2026/internal/office"

// Type aliases for animation state
type Position = office.Position
type AnimationState = office.AnimationState
type DeveloperAnimation = office.DeveloperAnimation
type OfficeState = office.OfficeState
type StaggeredAnimator = office.StaggeredAnimator

// Type aliases for events
type OfficeEvent = office.OfficeEvent
type DevAssignedToTicket = office.DevAssignedToTicket
type DevStartedWorking = office.DevStartedWorking
type DevBecameFrustrated = office.DevBecameFrustrated
type DevCompletedTicket = office.DevCompletedTicket
type DevEnteredConference = office.DevEnteredConference
type DevStartedMovingToConference = office.DevStartedMovingToConference
type AnimationFrameAdvanced = office.AnimationFrameAdvanced
type BubblesExpired = office.BubblesExpired

// Type alias for projection
type OfficeProjection = office.OfficeProjection

// Re-export constants
const (
	StateIdle               = office.StateIdle
	StateConference         = office.StateConference
	StateMovingToConference = office.StateMovingToConference
	StateMovingToCubicle    = office.StateMovingToCubicle
	StateWorking            = office.StateWorking
	StateFrustrated         = office.StateFrustrated
)

// Re-export layout constants
const (
	CubicleWidth     = office.CubicleWidth
	CubicleHeight    = office.CubicleHeight
	CubicleSpacing   = office.CubicleSpacing
	CubicleStartX    = office.CubicleStartX
	CubicleStartY    = office.CubicleStartY
	ConferenceX      = office.ConferenceX
	ConferenceY      = office.ConferenceY
	ConferenceWidth  = office.ConferenceWidth
	ConferenceHeight = office.ConferenceHeight
)

// Re-export variables
var (
	WorkingFrames         = office.WorkingFrames
	FrustratedFrames      = office.FrustratedFrames
	DeveloperColors       = office.DeveloperColors
	DeveloperColorNames   = office.DeveloperColorNames
	DefaultDeveloperNames = office.DefaultDeveloperNames
)

// Re-export functions
var (
	NewDeveloperAnimation = office.NewDeveloperAnimation
	NewOfficeState        = office.NewOfficeState
	NewOfficeProjection   = office.NewOfficeProjection
	CubicleLayout         = office.CubicleLayout
	ConferencePosition    = office.ConferencePosition
	Lerp                  = office.Lerp
	RenderDeveloperIcon   = office.RenderDeveloperIcon
	RenderCubicle         = office.RenderCubicle
	RenderConferenceRoom  = office.RenderConferenceRoom
	RenderCubicleGrid     = office.RenderCubicleGrid
	RenderCubicleCompact  = office.RenderCubicleCompact
	RenderOffice          = office.RenderOffice
	StripANSI             = office.StripANSI
)
