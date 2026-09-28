@gi @inference @codex
Feature: Send a Codex request accepted by the provider
  Piclaw 3.2.4's shipped Codex request builder omits max_output_tokens.
  The pinned Go provider adds this field even without a requested token limit.
  This compatibility contract applies only to the Codex Responses API.

  Scenario: Preserve payload hooks without sending an unsupported output limit
    Given a selected Codex Responses model and an authenticated session
    When the client builds a request and runs its configured payload hook
    Then the final Codex request omits max_output_tokens
    And the hook's other payload fields remain intact
    And a hook rejection prevents the request
    And ordinary OpenAI Responses requests retain their output limit

  Scenario: Complete a local Codex transport exchange
    Given an isolated provider fixture rejects max_output_tokens
    When the native inference path submits a prompt through Codex SSE fallback
    Then exactly one POST reaches the fixture without max_output_tokens
    And the streamed assistant text is returned successfully
    And no live credential or remote provider is used by the test
