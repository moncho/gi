Feature: Stored assistant Markdown in a real terminal
  A formatted assistant message must remain readable after layout, highlighting and resize.
  Captures are taken from tmux PTYs, not from a Markdown projector mock.

  Scenario Outline: Headings, emphasis and lists at <mode> <size>
    Given a stored assistant Markdown fixture "heading-list"
    When I open the transcript in <mode> at <size>
    Then headings emphasis and list bullets are projected without source markers
    And the Markdown remains readable after a resize with an unsent draft

    Examples:
      | mode       | size   |
      | fullscreen | 60x18  |
      | fullscreen | 100x22 |
      | fullscreen | 140x36 |
      | regular    | 60x18  |
      | regular    | 100x22 |
      | regular    | 140x36 |

  Scenario Outline: Inline highlighting and fenced code at <mode> <size>
    Given a stored assistant Markdown fixture "code-quote"
    When I open the transcript in <mode> at <size>
    Then inline code has ANSI highlighting without splitting or losing words
    And fenced code retains its four-space indentation and quote text
    And the Markdown remains readable after a resize with an unsent draft

    Examples:
      | mode       | size   |
      | fullscreen | 60x18  |
      | fullscreen | 100x22 |
      | fullscreen | 140x36 |
      | regular    | 60x18  |
      | regular    | 100x22 |
      | regular    | 140x36 |

  Scenario Outline: Tables and links at <mode> <size>
    Given a stored assistant Markdown fixture "table-link"
    When I open the transcript in <mode> at <size>
    Then table cells and the link target remain readable without raw Markdown
    And the Markdown remains readable after a resize with an unsent draft

    Examples:
      | mode       | size   |
      | fullscreen | 60x18  |
      | fullscreen | 100x22 |
      | fullscreen | 140x36 |
      | regular    | 60x18  |
      | regular    | 100x22 |
      | regular    | 140x36 |
