package mcp

import "github.com/lehmann314159/yrpg-backend/internal/game"

// These functions delegate to game.Format* so that mcp callers
// can use package-local names instead of game.Format* everywhere.

func FormatAttackResult(r *game.AttackResult) string       { return game.FormatAttackResult(r) }
func FormatRetreatResult(r *game.RetreatResult) string     { return game.FormatRetreatResult(r) }
func FormatTrapDetection(r *game.TrapDetectionResult) string { return game.FormatTrapDetection(r) }
func FormatTrapTrigger(r *game.TrapTriggerResult) string   { return game.FormatTrapTrigger(r) }
func FormatDisarmResult(r *game.TrapDisarmResult) string    { return game.FormatDisarmResult(r) }
func FormatSneakResult(r *game.SneakResult) string         { return game.FormatSneakResult(r) }
func FormatCombatHideResult(r *game.CombatHideResult) string { return game.FormatCombatHideResult(r) }
