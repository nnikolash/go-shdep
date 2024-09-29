package utils

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func Must2[Ret any](r Ret, err error) Ret {
	if err != nil {
		panic(err)
	}
	return r
}
