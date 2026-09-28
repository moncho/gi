@gi @provider-recovery @piclaw-oracle
Feature: Recover a pre-response provider timeout without ending the turn
  Installed Pi 0.87.1 and Piclaw 3.2.4 classify the reported Copilot HTTP/2
  response-header timeout as transient. Default backoff is 2, 4 and 8 seconds.
  Partial-output continuation and general automatic recovery need separate proof.

  Scenario: Recover on a later request within the same turn
    Given a running turn has submitted one user prompt
    And its provider fails before streaming text, thinking or tool calls
    When the failure is a retryable response-header timeout
    Then the turn stays running and shows the bounded retry attempt and delay
    And retry status is available from the authoritative activity snapshot
    And no terminal error post appears while retrying
    When a later request succeeds
    Then the original turn completes without a duplicate user prompt
    And retry status clears while newer composer text remains intact

  Scenario: Exhaust retries once
    Given the provider repeatedly times out before streaming progress
    When the three default retries have failed
    Then the turn fails with one durable system error notice
    And no further request is scheduled automatically
    And earlier tool actions are not replayed

  Scenario: Cancel during backoff
    Given the turn is waiting to retry
    When I cancel the active turn
    Then backoff ends promptly without another provider request
    And the authoritative state no longer offers active retry status

  Scenario Outline: Do not blindly repeat unsafe failures
    Given the attempt has <condition>
    When the provider request fails
    Then this pre-response retry path does not repeat the request

    Examples:
      | condition                     |
      | streamed text                 |
      | streamed thinking             |
      | a tool-call event             |
      | an authentication failure     |
      | invalid input                 |
      | a payload hook rejection      |
      | cancellation of the turn      |
    # This covers suppression of blind replay, not partial-output recovery.

  Scenario: Honour disabled retries
    Given Pi retry settings disable retries or set maxRetries to zero
    When the provider request fails
    Then this path does not schedule an automatic retry
