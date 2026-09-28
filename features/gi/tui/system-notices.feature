@tui @system-notices
Feature: Preserve system notices during a live turn
  Scenario: A provider error is not relabelled as an assistant reply
    Given the selected turn emits a provider error
    When its durable system_message post arrives with the same error
    Then the transcript has one error presentation
    And no assistant-labelled copy of the system error is added
    And reopening the durable system record preserves its system role

  Scenario: Normal system notices and assistant replies keep separate roles
    Given the selected chat receives a new_post event
    When its payload type is system_message
    Then the TUI appends a system notice rather than finalising an assistant draft
    When its payload type is agent_response
    Then the TUI finalises the assistant reply normally
