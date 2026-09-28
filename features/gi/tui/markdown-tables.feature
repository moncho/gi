@gi @tui @native-pty @markdown-tables
Feature: Width-aware streaming Markdown tables
  Scenario Outline: Stream and reload a table without broken columns
    Given a disposable native <mode> TUI at <size>
    When a local provider streams the table header, delimiter and split inline-code cells
    Then the live preview renders a wrapped grid with terminal-cell-aligned borders
    And wide and combining Unicode cells retain their content
    When the terminal resizes during the response
    Then the grid is reprojected without wrapping old borders
    And the unsent editor draft is preserved
    When the response finishes and the session reloads
    Then the completed table remains a grid
    And the stored assistant Markdown is unchanged

    Examples:
      | mode       | size    |
      | regular    | 60x24   |
      | regular    | 100x28  |
      | regular    | 140x36  |
      | fullscreen | 60x24   |
      | fullscreen | 100x28  |
      | fullscreen | 140x36  |
