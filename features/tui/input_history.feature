Feature: Session-scoped TUI editor history
  The cursor keys navigate submitted input, including local commands, without submitting drafts.

  Scenario: Up and Down recall history, preserve the unsent draft, and survive restart
    Given a fresh gi TUI workspace
    When I start the gi TUI in tmux
    When I type "/where" and press Enter
    And I type "/help" and press Enter
    When I type "unsent history draft" without submitting
    Then the editor should contain "unsent history draft"
    When I press Up
    Then the editor should contain "/help"
    When I press Up
    Then the editor should contain "/where"
    When I press Down
    Then the editor should contain "/help"
    When I press Down
    Then the editor should contain "unsent history draft"
    When I press Ctrl-U
    And I type "/fork @other" and press Enter
    Then the screen should contain "switched to @other"
    When I type "/where" and press Enter
    When I press Up
    Then the editor should contain "/where"
    When I press Ctrl-U
    And I type "/switch @agent" and press Enter
    When I press Up
    Then the editor should contain "/fork @other"
    When I press Ctrl-U
    When I restart the TUI in the same workspace
    When I press Up
    Then the editor should contain "/fork @other"
    When I press Ctrl-U
    And I type "/switch @other" and press Enter
    When I press Up
    Then the editor should contain "/switch @agent"
