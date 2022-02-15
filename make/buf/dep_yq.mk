# Settable
YQ_VERSION ?= v4.20.1

YQ_URL := https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_$(GOOS)_$(GOARCH)

ifeq ($(GOOS), windows)
	YQ_URL := $(YQ_URL).exe
endif

YQ := $(CACHE_VERSIONS)/yq/$(YQ_VERSION)
$(YQ):
	@rm -f $(CACHE_BIN)/yq
	@mkdir -p $(CACHE_BIN)
	curl -ssL --fail \
		https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_$(GOOS)_$(GOARCH) \
		-o $(CACHE_BIN)/yq
	chmod +x $(CACHE_BIN)/yq
	@rm -rf $(dir $(YQ))
	@mkdir -p $(dir $(YQ))
	@touch $(YQ)

.PHONY: goos
goos:
	@echo $(GOOS)

.PHONY: yq_url
yq_url:
	@echo $(YQ_URL)

