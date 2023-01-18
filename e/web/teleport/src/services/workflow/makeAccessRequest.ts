import { formatDistanceStrict } from 'date-fns';

import {
  AccessRequest,
  AccessRequestReview,
  AccessRequestReviewer,
} from './types';

export default function makeAccessRequest(json?): AccessRequest {
  json = json || {};

  const reviews = makeReviews(json.reviews);
  const reviewers = makeReviewers(json.suggestedReviewers, reviews);

  return {
    id: json.id,
    state: json.state,
    user: json.user,
    expires: json.expires,
    expiresDuration: getDurationText(json.expires),
    created: json.created,
    createdDuration: getDurationAgoText(json.created),
    roles: json.roles || [],
    resolveReason: json.resolveReason,
    requestReason: json.requestReason,
    reviews,
    reviewers,
    thresholdNames: json.thresholdNames || [],
    resources: json.resources || [],
  };
}

function makeReviews(jsonReviews): AccessRequestReview[] {
  jsonReviews = jsonReviews || [];

  return jsonReviews.map(review => ({
    author: review.author,
    state: review.state,
    reason: review.reason,
    roles: review.roles || [],
    createdDuration: getDurationAgoText(review.created),
  }));
}

function makeReviewers(jsonSuggestedReviewers, reviews: AccessRequestReview[]) {
  jsonSuggestedReviewers = jsonSuggestedReviewers || [];

  let allReviewers: AccessRequestReviewer[] = jsonSuggestedReviewers.map(
    name =>
      ({
        name,
        state: 'PENDING',
      } as AccessRequestReviewer)
  );

  // The reviewers in reviews list, may not be a part of the suggested reviewers list
  // b/c any user with permission can review a request.
  reviews.forEach(review => {
    const index = jsonSuggestedReviewers.indexOf(review.author);

    if (index > -1) {
      allReviewers[index].state = review.state;
    } else {
      allReviewers = [
        ...allReviewers,
        { name: review.author, state: review.state },
      ];
    }
  });

  return allReviewers;
}

function getDurationText(date: Date) {
  if (!date) {
    return '';
  }

  const duration = formatDistanceStrict(new Date(), new Date(date));
  return duration;
}

function getDurationAgoText(date: Date) {
  return date
    ? formatDistanceStrict(new Date(date), new Date(), { addSuffix: true })
    : '';
}
