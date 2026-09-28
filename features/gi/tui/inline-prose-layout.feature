@tui @screenshot-3139
Feature: Inline paths stay in reading order before tool output
  Scenario Outline: Render assistant prose and tool output at <mode> <size>
    Given an assistant reply has prose before and after the inline path "internal/tui"
    And the next two stored messages are shell tool results
    When I open the stored transcript in <mode> at <size>
    Then the assistant sentence stays in reading order with width-dependent wrapping
    And the two tool results immediately follow in stored order
    And the inline path appears once without a lost word boundary
    And assistant and tool output are not surrounded by decorative boxes
    And resizing preserves that reading order and an unsent draft

    Examples:
      | mode       | size   |
      | fullscreen | 60x32  |
      | fullscreen | 100x32 |
      | fullscreen | 140x32 |
      | regular    | 60x32  |
      | regular    | 100x32 |
      | regular    | 140x32 |
