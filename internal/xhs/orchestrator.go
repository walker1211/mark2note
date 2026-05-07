package xhs

import (
	"context"
	"fmt"
)

type Orchestrator struct {
	session    BrowserSession
	publisher  Publisher
	liveBridge LiveBridge
}

func NewOrchestrator(session BrowserSession) *Orchestrator {
	return &Orchestrator{session: session, publisher: Publisher{}, liveBridge: newDefaultLiveBridge()}
}

func (o *Orchestrator) Publish(ctx context.Context, request PublishRequest) (PublishResult, error) {
	defaultXHSLogger("publish start account=%s mode=%s media=%s", request.Account, request.Mode, request.MediaKind)
	result := PublishResult{
		TargetAccount: request.Account,
		Mode:          request.Mode,
		MediaKind:     request.MediaKind,
		ScheduleTime:  request.ScheduleTime,
	}
	defaultXHSLogger("publish open session")
	if err := o.session.Open(ctx); err != nil {
		return result, err
	}
	defaultXHSLogger("publish ensure login")
	if err := o.session.EnsureLoggedIn(ctx); err != nil {
		result.BrowserKept = true
		return result, err
	}
	defaultXHSLogger("publish get publisher page")
	page, err := o.session.PublisherPage(ctx)
	if err != nil {
		result.BrowserKept = true
		return result, err
	}
	if request.MediaKind == MediaKindLive {
		defaultXHSLogger("publish live attach prepare")
		attachRequest, err := buildLiveAttachRequest(request)
		if err != nil {
			result.BrowserKept = true
			return result, err
		}
		defaultXHSLogger("publish live attach run album=%s items=%d", attachRequest.AlbumName, len(attachRequest.Items))
		attachResult, err := o.liveBridge.Attach(ctx, attachRequest)
		if err != nil {
			result.BrowserKept = true
			return result, err
		}
		result.AttachedCount = attachResult.AttachedCount
		result.AttachedItems = append([]string(nil), attachResult.ItemNames...)
		defaultXHSLogger("publish live attach success attached=%d", attachResult.AttachedCount)
	}
	switch request.Mode {
	case PublishModeDraft:
		if err := o.publishDraft(ctx, page, request); err != nil {
			result.BrowserKept = true
			return result, err
		}
		result.DraftSaved = true
	case PublishModeSchedule:
		if err := o.publishScheduled(ctx, page, request); err != nil {
			result.BrowserKept = true
			return result, err
		}
	default:
		result.BrowserKept = true
		return result, fmt.Errorf("unsupported publish mode: %s", request.Mode)
	}
	if err := o.session.Close(); err != nil {
		return result, err
	}
	return result, nil
}

func (o *Orchestrator) publishDraft(ctx context.Context, page PublishPage, request PublishRequest) error {
	if request.MediaKind == MediaKindLive {
		if err := page.Open(ctx); err != nil {
			return err
		}
		if err := page.FillTitle(ctx, request.Title); err != nil {
			return fmt.Errorf("%w: %v", ErrFillFailed, err)
		}
		if err := page.FillContent(ctx, request.Content, request.Tags); err != nil {
			return fmt.Errorf("%w: %v", ErrFillFailed, err)
		}
		if err := page.SaveDraft(ctx); err != nil {
			return fmt.Errorf("%w: %v", ErrSubmitFailed, err)
		}
		if err := page.ConfirmDraftSaved(ctx); err != nil {
			return fmt.Errorf("%w: %v", ErrSubmitFailed, err)
		}
		return nil
	}
	return o.publisher.PublishStandardDraft(ctx, page, request)
}

func (o *Orchestrator) publishScheduled(ctx context.Context, page PublishPage, request PublishRequest) error {
	if request.MediaKind == MediaKindLive {
		if request.ScheduleTime == nil {
			return fmt.Errorf("%w: schedule time is required", ErrScheduleFailed)
		}
		if err := page.Open(ctx); err != nil {
			return err
		}
		if err := page.FillTitle(ctx, request.Title); err != nil {
			return fmt.Errorf("%w: %v", ErrFillFailed, err)
		}
		if err := page.FillContent(ctx, request.Content, request.Tags); err != nil {
			return fmt.Errorf("%w: %v", ErrFillFailed, err)
		}
		if err := page.SetSchedule(ctx, *request.ScheduleTime); err != nil {
			return fmt.Errorf("%w: %v", ErrScheduleFailed, err)
		}
		if err := page.SubmitScheduled(ctx); err != nil {
			return fmt.Errorf("%w: %v", ErrSubmitFailed, err)
		}
		if err := page.ConfirmScheduledSubmitted(ctx); err != nil {
			return fmt.Errorf("%w: %v", ErrSubmitFailed, err)
		}
		return nil
	}
	return o.publisher.PublishStandardScheduled(ctx, page, request)
}
