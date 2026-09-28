import {test,expect} from 'bun:test';
import {isAdaptiveCardSubmissionBlock,extractAdaptiveCardSubmissionBlocks} from '../../../web/src/ui/adaptive-card-submission.ts';

const valid={type:'adaptive_card_submission',card_id:'card-1',source_post_id:42,submitted_at:'2026-09-28T12:00:00.000Z',action_type:'Action.Submit'};
test('Classic002 submission identity requires bounded card ID, safe positive post, timestamp and Submit',()=>{
 expect(isAdaptiveCardSubmissionBlock(valid)).toBe(true);
 for(const block of [
  {...valid,card_id:''},{...valid,card_id:'   '},{...valid,card_id:'x'.repeat(257)},
  {...valid,source_post_id:0},{...valid,source_post_id:-1},{...valid,source_post_id:1.5},{...valid,source_post_id:Number.MAX_SAFE_INTEGER+1},
  {...valid,submitted_at:'bad-date'},{...valid,submitted_at:''},
  {...valid,action_type:'Action.OpenUrl'},
 ])expect(isAdaptiveCardSubmissionBlock(block)).toBe(false);
 expect(isAdaptiveCardSubmissionBlock({...valid,card_id:'x'.repeat(256),source_post_id:Number.MAX_SAFE_INTEGER})).toBe(true);
 expect(extractAdaptiveCardSubmissionBlocks([{...valid,action_type:undefined},{...valid,action_type:'Action.OpenUrl'}])).toEqual([valid]);
});
