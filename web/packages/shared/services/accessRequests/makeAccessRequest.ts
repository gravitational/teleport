/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { formatDistanceStrict } from 'date-fns';

import {
  AccessRequest,
  AccessRequestReview,
  AccessRequestReviewer,
  RequestKind,
} from './accessRequests';

// TODO(gzdunek): This function should live in the Web UI.
// As of now, it is also used by Connect,
// to allow it produce the full `AccessRequest` object.
// There are two problems with it:
// 1. In Connect we receive a typed gRPC response,
// so we don't need all these manual types conversions.
// 2. Many of the `AccessRequest` properties could be as well calculated
// in places where they are needed, instead of made in `makeAccessRequest`.
// For example, `requestTTLDuration`.

export function makeAccessRequest(json?): AccessRequest {
  json = json || {};

  // DELETE IN 20.0: Remove the fallback to the legacy user field.
  const user = json.userInfo?.username ?? json.user;
  const reviews = makeReviews(json.reviews);
  // DELETE IN 20.0: Remove the fallback to legacy suggested reviewers.
  const suggestedReviewersInfo =
    json.suggestedReviewersInfo ??
    (json.suggestedReviewers || []).map(username => ({ username }));
  const reviewers = makeReviewers(suggestedReviewersInfo, reviews);

  return {
    id: json.id,
    state: json.state,
    user,
    userDisplay: json.userInfo?.display,
    expires: new Date(json.expires),
    expiresDuration: getDurationText(json.expires),
    created: new Date(json.created),
    createdDuration: getDurationAgoText(json.created),
    // maxDuration can be null if talking with an older auth (before v13.3)
    maxDuration: json.maxDuration ? new Date(json.maxDuration) : null,
    maxDurationText: getDurationText(json.maxDuration),
    requestTTL: json.requestTTL,
    requestTTLDuration: getDurationText(json.requestTTL),
    // sessionTTL can be null if talking with an older auth (before v13.3)
    sessionTTL: json.sessionTTL ? new Date(json.sessionTTL) : null,
    sessionTTLDuration: getDurationText(json.sessionTTL),
    roles: json.roles || [],
    resolveReason: json.resolveReason,
    requestReason: json.requestReason,
    reviews,
    reviewers,
    thresholdNames: json.thresholdNames || [],
    resources: json.resources || [],
    promotedAccessListTitle: json.promotedAccessListTitle,
    // assumeStartTime can be null because it's an optional field
    // to request.
    assumeStartTime: json.assumeStartTime
      ? new Date(json.assumeStartTime)
      : null,
    assumeStartTimeDuration: getAssumeStartDurationText(json.assumeStartTime),
    reasonMode: json.reasonMode || 'optional',
    reasonPrompts: json.reasonPrompts || [],
    requestKind: getRequestKind(json.requestKind),
    longTermResourceGrouping: json.longTermResourceGrouping,
  };
}

function getRequestKind(jsonKind: unknown): RequestKind {
  if (typeof jsonKind !== 'number') {
    return RequestKind.Undefined;
  }
  return jsonKind === RequestKind.LongTerm
    ? RequestKind.LongTerm
    : jsonKind === RequestKind.ShortTerm
      ? RequestKind.ShortTerm
      : RequestKind.Undefined;
}

function makeReviews(jsonReviews): AccessRequestReview[] {
  jsonReviews = jsonReviews || [];

  return jsonReviews.map(review => {
    // DELETE IN 20.0: Remove the fallback to the legacy author field.
    const author = review.authorInfo?.username ?? review.author;

    return {
      author,
      authorDisplay: review.authorInfo?.display,
      state: review.state,
      reason: review.reason,
      roles: review.roles || [],
      createdDuration: getDurationAgoText(review.created),
      promotedAccessListTitle: review.promotedAccessListTitle,
      assumeStartTime: review.assumeStartTime
        ? new Date(review.assumeStartTime)
        : null,
    };
  });
}

function makeReviewers(
  jsonSuggestedReviewersInfo,
  reviews: AccessRequestReview[]
) {
  jsonSuggestedReviewersInfo = jsonSuggestedReviewersInfo || [];

  let allReviewers: AccessRequestReviewer[] = jsonSuggestedReviewersInfo.map(
    reviewer =>
      ({
        name: reviewer.username,
        display: reviewer.display,
        state: 'PENDING',
      }) as AccessRequestReviewer
  );

  // The reviewers in reviews list, may not be a part of the suggested reviewers list
  // b/c any user with permission can review a request.
  reviews.forEach(review => {
    const index = jsonSuggestedReviewersInfo.findIndex(
      reviewer => reviewer.username === review.author
    );

    if (index > -1) {
      allReviewers[index].state = review.state;
      allReviewers[index].display ??= review.authorDisplay;
    } else {
      allReviewers = [
        ...allReviewers,
        {
          name: review.author,
          display: review.authorDisplay,
          state: review.state,
        },
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

function getAssumeStartDurationText(date: Date) {
  if (canAssumeNow(date)) {
    return 'now';
  }

  return `${getDurationText(date)} from now`;
}

export function canAssumeNow(date: Date) {
  if (!date) {
    return true;
  }

  return Date.now() >= new Date(date).getTime();
}
