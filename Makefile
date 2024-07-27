EXEC_TARGET := $(notdir $(shell pwd))
SUBEXEC_TARGET := 

IS_DEBUG := 1

MODULE_PREFIX := github.com/ptquang2000
BUILD_DIR := ./build

GO_SRCS := $(shell find ./src -iname '*.go')
GO_BUILD := go build
GO_BUILD_FLAGS :=
GO_CLEAN := go clean -testcache

module_path = $(subst build, $(MODULE_PREFIX), $(1))
submodule_path = $(subst build, $(MODULE_PREFIX)/$(EXEC_TARGET), $(1))

export DEBUG := $(IS_DEBUG)

.PHONY: all
all: build
	$(BUILD_DIR)/$(EXEC_TARGET)

.PHONY: build
ifeq (${SUBEXEC_TARGET},)
build: $(BUILD_DIR)/$(EXEC_TARGET)
else
build: $(addprefix $(BUILD_DIR)/, $(EXEC_TARGET) $(SUBEXEC_TARGET))
endif

$(BUILD_DIR)/$(EXEC_TARGET): main.go $(GO_SRCS)
	mkdir -p $(dir $@)
	$(GO_BUILD) -o $@ $(GO_BUILD_FLAGS) $(call module_path, $@)

$(BUILD_DIR)/%: %/main.go $(GO_SRCS)
	mkdir -p $(dir $@)
	$(GO_BUILD) -o $@ $(GO_BUILD_FLAGS) $(call submodule_path, $@)

GO_TEST := go test -v ./...

.PHONY: test
test:
	$(GO_TEST)

Test%:
	$(GO_TEST) -run $@

.PHONY: clean
clean:
	$(GO_CLEAN)
	rm -rf $(BUILD_DIR)
	
