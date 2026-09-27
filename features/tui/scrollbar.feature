Feature: Scroll the fullscreen transcript with the mouse in tmux
  The scrollbar gutter is interactive without selecting text or changing the draft.

  Scenario Outline: Track click and thumb drag at <size>
    Given a long stored transcript in a <size> fullscreen tmux pane
    When I click the scrollbar track
    Then the transcript jumps toward the middle
    When I drag the scrollbar thumb above the top of the pane
    Then the transcript shows its first marker
    When I drag the scrollbar thumb below the bottom of the pane
    Then the transcript shows its last marker
    And my unsent draft is unchanged

    Examples:
      | size   |
      | 60x18  |
      | 100x22 |
      | 140x36 |
