package xhs

import (
	"context"
	"fmt"

	"github.com/walker1211/mark2note/internal/timing"
)

type Orchestrator struct {
	session    BrowserSession
	publisher  Publisher
	liveBridge LiveBridge
}

func NewOrchestrator(session BrowserSession) *Orchestrator {
	return &Orchestrator{session: session, publisher: Publisher{}, liveBridge: newDefaultLiveBridge()}
}

func (o *Orchestrator) Publish(ctx context.Context, request PublishRequest) (result PublishResult, err error) {
	done := timing.Stage("xhs.Orchestrator.Publish", timing.Field("mode", request.Mode), timing.Field("media", request.MediaKind))
	defer func() { done(err) }()

	defaultXHSLogger("publish start account=%s mode=%s media=%s", request.Account, request.Mode, request.MediaKind)
	result = PublishResult{
		TargetAccount: request.Account,
		Mode:          request.Mode,
		MediaKind:     request.MediaKind,
		ScheduleTime:  request.ScheduleTime,
	}
	defaultXHSLogger("publish open session")
	openDone := timing.Stage("xhs.Orchestrator.session_open")
	err = o.session.Open(ctx)
	openDone(err)
	if err != nil {
		return result, err
	}
	defaultXHSLogger("publish ensure login")
	loginDone := timing.Stage("xhs.Orchestrator.ensure_login")
	err = o.session.EnsureLoggedIn(ctx)
	loginDone(err)
	if err != nil {
		result.BrowserKept = true
		return result, err
	}
	defaultXHSLogger("publish get publisher page")
	pageDone := timing.Stage("xhs.Orchestrator.get_publisher_page")
	page, err := o.session.PublisherPage(ctx)
	pageDone(err)
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
		attachDone := timing.Stage("xhs.Orchestrator.live_attach", timing.Field("items", len(attachRequest.Items)))
		attachResult, err := o.liveBridge.Attach(ctx, attachRequest)
		attachDone(err)
		if err != nil {
			result.BrowserKept = true
			return result, err
		}
		result.AttachedCount = attachResult.AttachedCount
		result.AttachedItems = append([]string(nil), attachResult.ItemNames...)
		defaultXHSLogger("publish live attach success attached=%d", attachResult.AttachedCount)
	}
	switch request.Mode {
	case PublishModeOnlySelf:
		publishDone := timing.Stage("xhs.Orchestrator.publish_only_self", timing.Field("media", request.MediaKind))
		err = o.publishOnlySelf(ctx, page, request)
		publishDone(err)
		if err != nil {
			result.BrowserKept = true
			return result, err
		}
		if !request.StopBeforeSubmit {
			result.OnlySelfPublished = true
		}
	case PublishModeSchedule:
		publishDone := timing.Stage("xhs.Orchestrator.publish_scheduled", timing.Field("media", request.MediaKind))
		err = o.publishScheduled(ctx, page, request)
		publishDone(err)
		if err != nil {
			result.BrowserKept = true
			return result, err
		}
	default:
		result.BrowserKept = true
		return result, fmt.Errorf("unsupported publish mode: %s", request.Mode)
	}
	if request.StopBeforeSubmit {
		result.StoppedBeforeSubmit = true
		result.BrowserKept = true
		return result, nil
	}
	closeDone := timing.Stage("xhs.Orchestrator.session_close")
	err = o.session.Close()
	closeDone(err)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (o *Orchestrator) publishOnlySelf(ctx context.Context, page PublishPage, request PublishRequest) error {
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
		if request.StopBeforeSubmit {
			return prepareOnlySelfBeforeSubmit(page, request)
		}
		if err := page.PublishOnlySelf(ctx, request); err != nil {
			return fmt.Errorf("%w: %v", ErrSubmitFailed, err)
		}
		if err := page.ConfirmOnlySelfPublished(ctx); err != nil {
			return fmt.Errorf("%w: %v", ErrSubmitFailed, err)
		}
		return nil
	}
	return o.publisher.PublishStandardOnlySelf(ctx, page, request)
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
		if err := runScheduledPreSubmitHooks(page, request); err != nil {
			return err
		}
		if err := page.SetSchedule(ctx, *request.ScheduleTime); err != nil {
			return fmt.Errorf("%w: %v", ErrScheduleFailed, err)
		}
		if request.StopBeforeSubmit {
			return nil
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
