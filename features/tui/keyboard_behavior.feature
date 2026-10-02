Feature: TUI keyboard behavior
  The gi TUI should support predictable keyboard-only operation in tmux.

  Scenario: Idle Escape keeps editor focus, history, scroll, resize, and quit
    Given a fresh gi TUI workspace
    When I start the gi TUI in tmux
    Then the screen should contain "Press Ctrl+O to show full startup help and loaded resources."
    When I press Escape
    And I type "still focused" and press Enter
    Then the database should contain a user message "still focused"
    Then the screen should contain "Gi received: still focused"
    When I press PageUp
    Then the screen should contain "Press Ctrl+O to show full startup help and loaded resources."
    When I press End
    Then the screen should contain "Gi received: still focused"
    When I resize the terminal to 100x22
    Then the tmux session should be alive
    And the screen should contain "Gi received: still focused"
    When I resize the terminal to 60x18
    Then the tmux session should be alive
    And the screen should contain "Gi received: still focused"
    When I press Ctrl-D
    Then the tmux session should exit
