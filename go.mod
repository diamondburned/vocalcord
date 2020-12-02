module github.com/diamondburned/vocalcord

go 1.15

//replace github.com/diamondburned/arikawa/v2 => ../arikawa

replace github.com/gotk3/gotk3 => github.com/diamondburned/gotk3 v0.0.0-20201130155633-7bac31bb1d45

require (
	github.com/diamondburned/arikawa/v2 v2.0.0-20201202020742-5e2af90fd009
	github.com/gotk3/gotk3 v0.5.1 // indirect
	github.com/hajimehoshi/oto v0.6.8
	github.com/pkg/errors v0.9.1
	layeh.com/gopus v0.0.0-20161224163843-0ebf989153aa
)
