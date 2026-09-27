Feature: Session-scoped TUI editor history
  The cursor keys navigate submitted prompts per session, without submitting drafts or commands.

  Scenario: Up and Down recall history, preserve the unsent draft, and survive restart
    Given a fresh gi TUI workspace
    When I start the gi TUI in tmux
    When I type "first prompt" and press Enter
    Then the database should contain a user message "first prompt"
    When I type "second prompt" and press Enter
    Then the database should contain a user message "second prompt"
    When I type "unsent history draft" without submitting
    Then the editor should contain "unsent history draft"
    When I press Up
    Then the editor should contain "second prompt"
    When I press Up
    Then the editor should contain "first prompt"
    When I press Down
    Then the editor should contain "second prompt"
    When I press Down
    Then the editor should contain "unsent history draft"
    When I press Ctrl-U
    And I type "/fork @other" and press Enter
    Then the screen should contain "switched to @other"
    When I type "@other peer prompt" and press Enter
    Then the database should contain a user message "peer prompt"
    When I press Up
    Then the editor should contain "@other peer prompt"
    When I press Ctrl-U
    And I type "/switch @agent" and press Enter
    When I press Up
    Then the editor should contain "second prompt"
    When I press Ctrl-U
    When I restart the TUI in the same workspace
    When I press Up
    Then the editor should contain "second prompt"
    When I press Ctrl-U
    And I type "/switch @other" and press Enter
    When I press Up
    Then the editor should contain "@other peer prompt"
