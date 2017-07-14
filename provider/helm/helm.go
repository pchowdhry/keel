package helm

import (
	"fmt"

	"github.com/rusenask/keel/types"
	"github.com/rusenask/keel/util/image"

	hapi_chart "k8s.io/helm/pkg/proto/hapi/chart"
	rls "k8s.io/helm/pkg/proto/hapi/services"

	log "github.com/Sirupsen/logrus"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
)

const ProviderName = "helm"

type Provider struct {
	implementer Implementer

	events chan *types.Event
	stop   chan struct{}
}

func NewProvider(implementer Implementer) *Provider {
	return &Provider{
		implementer: implementer,
		events:      make(chan *types.Event, 100),
		stop:        make(chan struct{}),
	}
}

func (p *Provider) GetName() string {
	return ProviderName
}

// Submit - submit event to provider
func (p *Provider) Submit(event types.Event) error {
	p.events <- &event
	return nil
}

// Start - starts kubernetes provider, waits for events
func (p *Provider) Start() error {
	return p.startInternal()
}

// Stop - stops kubernetes provider
func (p *Provider) Stop() {
	close(p.stop)
}

func (p *Provider) startInternal() error {
	for {
		select {
		case event := <-p.events:
			log.WithFields(log.Fields{
				"repository": event.Repository.Name,
				"tag":        event.Repository.Tag,
				"registry":   event.Repository.Host,
			}).Info("provider.helm: processing event")
			err := p.processEvent(event)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
					"image": event.Repository.Name,
					"tag":   event.Repository.Tag,
				}).Error("provider.helm: failed to process event")
			}
		case <-p.stop:
			log.Info("provider.helm: got shutdown signal, stopping...")
			return nil
		}
	}
}

func (p *Provider) processEvent(event *types.Event) (err error) {

	return nil
}

func (p *Provider) getImpactedReleases(event *types.Event) ([]*rls.ListReleasesResponse, error) {
	releaseList, err := p.implementer.ListReleases()
	if err != nil {
		return nil, err
	}

	for _, release := range releaseList.Releases {

		ref, err := parseImage(release.Chart, release.Config)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("provider.helm: failed to get image name and tag from release")
			continue
		}
		log.WithFields(log.Fields{
			"parsed_image_named": ref.Remote(),
		}).Info("provider.helm: checking image")

	}

	return nil, nil
}

// resp, err := u.client.UpdateRelease(
// 		u.release,
// 		chartPath,
// 		helm.UpdateValueOverrides(rawVals),
// 		helm.UpgradeDryRun(u.dryRun),
// 		helm.UpgradeRecreate(u.recreate),
// 		helm.UpgradeForce(u.force),
// 		helm.UpgradeDisableHooks(u.disableHooks),
// 		helm.UpgradeTimeout(u.timeout),
// 		helm.ResetValues(u.resetValues),
// 		helm.ReuseValues(u.reuseValues),
// 		helm.UpgradeWait(u.wait))
// 	if err != nil {
// 		return fmt.Errorf("UPGRADE FAILED: %v", prettyError(err))
// 	}

func updateHelmRelease(implementer Implementer, releaseName string, chart *hapi_chart.Chart, rawVals string) error {

	resp, err := implementer.UpdateReleaseFromChart(releaseName, chart,
		helm.UpdateValueOverrides([]byte(rawVals)),
		helm.UpgradeDryRun(false),
		helm.UpgradeRecreate(false),
		helm.UpgradeForce(true),
		helm.UpgradeDisableHooks(false),
		helm.UpgradeTimeout(30),
		helm.ResetValues(false),
		helm.ReuseValues(true),
		helm.UpgradeWait(true))

	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"version": resp.Release.Version,
		"release": releaseName,
	}).Info("provider.helm: release updated")
	return nil
}

func parseImage(chart *hapi_chart.Chart, config *hapi_chart.Config) (*image.Reference, error) {
	vals, err := chartutil.ReadValues([]byte(config.Raw))
	if err != nil {
		return nil, err
	}

	log.Info(config.Raw)

	imageName, err := vals.PathValue("image.repository")
	if err != nil {
		return nil, err
	}

	imageTag, err := vals.PathValue("image.tag")
	if err != nil {
		return nil, fmt.Errorf("failed to get image tag: %s", err)
	}

	imageNameStr, ok := imageName.(string)
	if !ok {
		return nil, fmt.Errorf("failed to convert image name ref to string")
	}

	imageTagStr, ok := imageTag.(string)
	if !ok {
		return nil, fmt.Errorf("failed to convert image tag ref to string")
	}

	if imageTagStr != "" {
		return image.Parse(imageNameStr + ":" + imageTagStr)
	}

	return image.Parse(imageNameStr)
}
