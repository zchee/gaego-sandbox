# ----------------------------------------------------------------------------
# global

.DEFAULT_GOAL := static
APP = gaego-sandbox
CMD_PREFIX =
CMD = .

# ----------------------------------------------------------------------------
# target

deploy:
	gcloud --project=$(GOOGLE_CLOUD_PROJECT) beta app deploy --promote --stop-previous-version --version='master' --quiet --verbosity='debug' app.yaml

deploy/%:
	gcloud --project=$(GOOGLE_CLOUD_PROJECT) beta app deploy --promote --stop-previous-version --version='$@' --quiet --verbosity='debug' app.yaml

# ----------------------------------------------------------------------------
# include

include hack/make/go.mk
