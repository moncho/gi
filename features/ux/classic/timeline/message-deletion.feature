@classic @piclaw-3.2.4 @source-reviewed
Feature: Message deletion from timeline
  Installed Piclaw 3.2.4 counts replies in the loaded view before prompting.
  Direct deletion does not guard against replies outside that view. The
  historical 70d33bc clauses are in features/ux/upstream/classic-snapshot/.

  Background:
    Given I am authenticated and on the main chat
    And the timeline contains messages

  Rule: Direct deletion follows the loaded timeline view
    @ux-timeline-017
    Scenario: Delete a single message without visible replies
      Given a message with no visible thread replies exists
      When I click the delete button on that message
      Then the message should enter the removing state
      And the message should be removed from the DOM after the removal delay
      And refreshing the page should not show the deleted message

    @ux-timeline-018 @oracle-mismatch
    Scenario: Direct deletion with an unseen stored reply leaves that reply behind
      Given a parent appears to have no replies in the loaded timeline view
      And the installed backend has a stored reply to that parent
      When I click the delete button on the parent
      Then the client requests direct deletion without a cascade prompt
      And the backend returns only the parent ID as deleted
      And the parent's row is removed while the stored reply retains its thread ID
      # Joined disposable UI/backend-function evidence; production HTTP/auth
      # and live deletion remain unverified. This is a known orphaning risk.

    @ux-timeline-019 @conditional-fixture
    Scenario: A synthetic Replies exist rejection exposes the otherwise dormant retry prompt
      Given a parent appears to have no replies in the loaded timeline view
      And a disposable API fixture rejects direct deletion with "Replies exist"
      When I click the delete button on the parent
      Then the Classic UI asks whether to delete it and its replies
      And confirming retries with cascade enabled
      And cancelling makes no cascade request and leaves the parent visible
      # Installed Piclaw 3.2.4 direct deletion does not emit this rejection.
      # This tests a conditional UI branch, not a normal backend journey.

  Rule: Visible thread replies require explicit cascade confirmation
    @ux-timeline-020
    Scenario: Deleting a message with 3 visible replies asks for cascade confirmation
      Given a message that has 3 visible thread replies exists
      When I click the delete button on the parent message
      Then a confirmation prompt should ask "Delete this message and its 3 replies?"

    @ux-timeline-021
    Scenario: Confirming cascade deletes the parent and visible replies together
      Given a message that has visible thread replies exists
      When I click the delete button on the parent message
      And I confirm the cascade prompt
      Then the parent message should enter the removing state
      And its visible thread replies should enter the removing state
      And the parent and replies should be removed together

    @ux-timeline-022
    Scenario: Cancelling cascade preserves the parent and visible replies
      Given a message that has visible thread replies exists
      When I click the delete button on the parent message
      And I cancel the cascade prompt
      Then the parent message should remain visible
      And its visible thread replies should remain visible
