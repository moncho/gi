Feature: TUI settings and approval visibility
  The gi TUI should make runtime settings and approval-gate state discoverable.

  Scenario: Inspect runtime settings and approval state
    Given a fresh gi TUI workspace
    When I start the gi TUI in tmux
    Then the screen should contain "Press Ctrl+O to show full startup help and loaded resources."
    When I type "/settings" and press Enter
    Then the screen should contain "settings: provider retry"
    And the screen should contain "max_agent_delay_ms:"
    When I press Home
    Then the screen should contain "Press Ctrl+O to show full startup help and loaded resources."
    And the screen should contain "settings: runtime"
    When I press PageDown
    Then the screen should contain "settings: editor"
    And the screen should contain "theme_configured:"
    When I press End
    And I type "/approvals" and press Enter
    Then the screen should contain "approvals: no approval gates are configured in gi yet"
