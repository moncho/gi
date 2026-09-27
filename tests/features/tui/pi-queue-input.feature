@tui @pi-0.87.1 @queue
Feature: Distinguish steering from follow-up input
  Pi's interactive input and queue methods are the reference for terminal controls.
  Piclaw's web queue actions have a separate contract; they must not be used to
  justify changing Pi's editor keys or queue restoration semantics.

  Scenario: Enter steers a running turn while Alt+Enter queues a follow-up
    Given the selected session has a running turn
    And the composer has a persisted draft
    When I press Enter
    Then that captured draft is admitted as steering for the active turn
    When I type another draft and press Alt+Enter
    Then that captured draft is admitted as a separate follow-up
    And it is not consumed as steering by the active turn
    And the two submissions retain the selected session and model ownership

  Scenario: Alt+Enter without an active turn sends normally
    Given the selected session is idle
    When I type a prompt and press Alt+Enter
    Then the prompt can start a normal turn without waiting for a nonexistent active run

  Scenario: Shift+Enter remains a newline
    Given the composer contains a draft
    When I press Shift+Enter
    Then a newline is inserted without steering or queue admission

  @gap
  Scenario: Alt+Up restores all pending messages before the current draft
    Given the session has pending steering and follow-up messages
    And the editor contains newer text
    When I press Alt+Up
    Then the pending steering text followed by follow-up text precedes the newer draft
    And the pending messages are removed from their delivery queues
    And no restored message can also execute from its old queue entry
    # Installed Pi method oracle confirms text ordering/clear calls.
    # Gi's durable atomic restoration and terminal keyboard journey are not implemented.
