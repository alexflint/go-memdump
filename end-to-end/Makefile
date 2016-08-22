default: test

image:
	docker build -t alexflint/memdump-end-to-end .

test: image
	docker run -it --rm alexflint/memdump-end-to-end
