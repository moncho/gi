@gi-native @derived-from-piclaw-3.2.4 @not-frozen-parity
Feature: Fresh browser chat admission and durable reply in Gi
  The Piclaw 3.2.4 oracle establishes selected-chat composer dispatch and
  clearing, but its isolated fixture cannot prove Gi's native turn. These
  scenarios require the real disposable Gi API, SSE, store and provider fixture.

  @gi-basic-001
  Scenario: First Return admits exactly one turn and preserves its session on reload
    Given the browser has no selected chat or saved draft
    When Gi creates the first native session and focuses its composer
    And I type a unique prompt and press Return once
    Then one native prompt POST targets the created session with that exact text
    And its admitted turn ID reaches completed status with one matching user and assistant message
    And the visible timeline shows that user message and reply
    And reloading keeps the same selected session without resubmitting the prompt

  @gi-basic-002
  Scenario: Rejected first admission keeps the text for an explicit retry
    Given the first Gi session is ready and contains a unique unsent prompt
    When the native prompt admission rejects with a server error
    Then the composer retains the exact text and announces the failure
    And no native turn is created
    When I retry explicitly and the next admission is held
    Then repeated Return presses cannot admit duplicate turns
    And the accepted retry creates exactly one completed turn for that text
