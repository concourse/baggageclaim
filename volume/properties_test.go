package volume_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/baggageclaim/volume"
)

var _ = Describe("Properties Superset", func() {
	It("return true when the two sets are equal", func() {
		properties := volume.Properties{
			"name": "value",
		}

		result := properties.HasProperties(properties)
		Ω(result).Should(BeTrue())
	})

	It("returns true if all of the elements in the query are contained in the properties", func() {
		properties := volume.Properties{
			"name1": "value1",
			"name2": "value2",
		}

		query := volume.Properties{
			"name1": "value1",
		}

		result := properties.HasProperties(query)
		Ω(result).Should(BeTrue())
	})

	It("returns false if the query has more elements than the properties", func() {
		properties := volume.Properties{
			"name1": "value1",
		}

		query := volume.Properties{
			"name1": "value1",
			"name2": "value2",
		}

		result := properties.HasProperties(query)
		Ω(result).Should(BeFalse())
	})

	It("returns false if all of the names in the query are not contained in the properties", func() {
		properties := volume.Properties{
			"name1": "value1",
		}

		query := volume.Properties{
			"name2": "value1",
		}

		result := properties.HasProperties(query)
		Ω(result).Should(BeFalse())
	})

	It("returns false if all of the names and values in the query are not contained in the properties", func() {
		properties := volume.Properties{
			"name1": "value1",
		}

		query := volume.Properties{
			"name1": "value2",
		}

		result := properties.HasProperties(query)
		Ω(result).Should(BeFalse())
	})

	It("returns false if there is a query entry that does not exist in the properties", func() {
		properties := volume.Properties{
			"name1": "value1",
			"name2": "value2",
		}

		query := volume.Properties{
			"name1": "value1",
			"name3": "value3",
		}

		result := properties.HasProperties(query)
		Ω(result).Should(BeFalse())
	})

	It("returns true if the query and properties are empty", func() {
		properties := volume.Properties{}
		query := volume.Properties{}

		result := properties.HasProperties(query)
		Ω(result).Should(BeTrue())
	})

	It("returns true if the query is empty but properties are not", func() {
		properties := volume.Properties{
			"name1": "value1",
			"name2": "value2",
		}
		query := volume.Properties{}

		result := properties.HasProperties(query)
		Ω(result).Should(BeTrue())
	})
})
